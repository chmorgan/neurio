/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

/* Copyright 2017 Chris Morgan <chmorgan@gmail.com> */

package neurio

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/chmorgan/semaphore"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

/* NOTES:
 * - Occasionally the Neurio will return what appears to be a misaligned
 *   value for a given sample. This may be seen as a Exxx_Ws value that has
 *   changed roughly 2x what you might expect from a sample 1 second ago.
 *
 *   As a consequence, for deltas between samples 1 second apart, please use
 *  the instantanious values such as P_W rather than calculating using delta-Ws
 */

/*

Example json payload from http://xxx/current-sample/

{"sensorId":"0x0000C47F51019E05","timestamp":"2017-06-26T00:30:26Z",
"channels":[{"type":"PHASE_A_CONSUMPTION","ch":1,"eImp_Ws":21837827195,
"eExp_Ws":112135409,"p_W":1121,"q_VAR":15,"v_V":123.574},
{"type":"PHASE_B_CONSUMPTION","ch":2,"eImp_Ws":20624834816,"eExp_Ws":123249487,
"p_W":1428,"q_VAR":314,"v_V":123.729},
{"type":"NET","ch":3,"eImp_Ws":42045777679,
"eExp_Ws":1289626261,"p_W":2555,"q_VAR":293,"v_V":123.655},
{"type":"GENERATION","ch":4,"eImp_Ws":1497258783,"eExp_Ws":6754561,"p_W":-6,
"q_VAR":36,"v_V":123.648},{"type":"CONSUMPTION","ch":5,"eImp_Ws":1361713487,
"eExp_Ws":199739472,"p_W":2549,"q_VAR":329,"v_V":123.652}],
"cts":
[{"ct":1,"p_W":1127,"q_VAR":-3,"v_V":123.576},
{"ct":2,"p_W":1428,"q_VAR":296,"v_V":123.735},
{"ct":3,"p_W":1,"q_VAR":18,"v_V":123.723},
{"ct":4,"p_W":-6,"q_VAR":17,"v_V":123.573}]
}

*/

type Channel struct {
	ChannelType string  `json:"type"`
	Ch          int     `json:"ch"`
	EImp_Ws     int64   `json:"eImp_Ws"`
	EExp_Ws     int64   `json:"eExp_Ws"`
	P_W         int     `json:"p_W"`
	Q_VAR       int     `json:"q_VAR"`
	V_V         float64 `json:"v_V"`
}

type CT struct {
	CT    int     `json:"ct"`
	P_W   int     `json:"p_W"`
	Q_VAR int     `json:"q_VAR"`
	V_V   float64 `json:"v_V"`
}

/* See https://mholt.github.io/json-to-go/ for help */
type CurrentSampleResponse struct {
	SensorId  string    `json:"sensorId"`
	Timestamp time.Time `json:"timestamp"`
	Channels  []Channel `json:"channels"`
	CTs       []CT      `json:"cts"`
}

func (c *CurrentSampleResponse) FindChannelByType(channelType string) *Channel {
	for _, channel := range c.Channels {
		if channel.ChannelType == channelType {
			return &channel
		}
	}

	return nil
}

/*
 Assumes that the samples are ordered from newest to oldest, with the newest one first
 Using the first sample, locate and return the sample closest to the duration and
 the duration between the initial sample and the nearest sample.

 If there are less than 2 samples then return (nil, nil), there is no nearest sample.
*/
func FindNearestSampleForDuration(durationString string, samples []CurrentSampleResponse, logger log.Logger) (nearestSample CurrentSampleResponse, actualDuration time.Duration, err error) {
	duration, err := time.ParseDuration(durationString)

	if len(samples) < 2 {
		err = errors.New("not enough samples")
		return
	}

	if err != nil {
		level.Error(logger).Log("err", err)
	}

	initialSample := samples[0]

	// NOTE: We always start with the nearestSample as the next sample to ensure
	// that we are calculating a delta between two different samples and not
	// the same one
	nearestSample = samples[1]
	actualDuration = initialSample.Timestamp.Sub(nearestSample.Timestamp)

	// find the sample closest to the duration
	for _, sample := range samples {
		level.Debug(logger).Log("actualDuration", actualDuration)

		sampleDuration := initialSample.Timestamp.Sub(sample.Timestamp)
		level.Debug(logger).Log("sampleDuration", sampleDuration)

		// if we are still less than our desired duration
		if sampleDuration <= duration {
			// and we either haven't found a duration yet, or
			// the new sample is closer to our desired duration then
			// update our nearestSample with the current sample
			if sampleDuration > actualDuration {
				nearestSample = sample
				actualDuration = sampleDuration
			}
		} else {
			// NOTE: We can break out here if we assume 'samples' is strictly ordered from newest to oldest
			break
		}
	}

	level.Info(logger).Log("nearestSample.Timestamp", nearestSample.Timestamp, "actualDuration", actualDuration)

	return nearestSample, actualDuration, nil
}

/**
 * Calculate the exported watts on a given channel, this is
 * the exported watt seconds less the imported watt seconds divided by a number
 * of seconds to convert back to watts.
 */
func ExportedWattsForChannel(channelName string, duration time.Duration, deltaSample CurrentSampleResponse, logger log.Logger) (exportedWatts float64) {
	level.Debug(logger).Log("duration.Seconds()", duration.Seconds())

	for index, channel := range deltaSample.Channels {
		level.Debug(logger).Log("index", index, "ChannelType", channel.ChannelType,
			"Channel", channel.Ch,
			"EImp_Ws", channel.EImp_Ws,
			"EExp_Ws", channel.EExp_Ws)

		if channel.ChannelType == channelName {
			exportedWattSeconds := channel.EExp_Ws - channel.EImp_Ws
			level.Debug(logger).Log("exported_ws", exportedWattSeconds, "seconds", duration.Seconds())
			exportedWatts = float64(exportedWattSeconds) / duration.Seconds()
			level.Debug(logger).Log("exported watts", exportedWatts)
		}
	}

	return
}

/* returns (a - b) */
func DeltaSample(a CurrentSampleResponse, b CurrentSampleResponse, logger log.Logger) CurrentSampleResponse {
	var response CurrentSampleResponse

	response.SensorId = "0"
	response.Timestamp = a.Timestamp

	response.Channels = make([]Channel, len(a.Channels))
	response.CTs = make([]CT, len(a.CTs))

	for channelIndex := 0; channelIndex < len(a.Channels); channelIndex++ {
		a_chan := a.Channels[channelIndex]
		b_chan := b.Channels[channelIndex]
		response_chan := &response.Channels[channelIndex]

		response_chan.ChannelType = a_chan.ChannelType
		response_chan.Ch = a_chan.Ch

		response_chan.EImp_Ws = a_chan.EImp_Ws - b_chan.EImp_Ws
		response_chan.EExp_Ws = a_chan.EExp_Ws - b_chan.EExp_Ws
		//        level.Info(logger).Log("i_a", a_chan.EImp_Ws, "i_b", b_chan.EImp_Ws, "e_a", a_chan.EExp_Ws, "e_b", b_chan.EExp_Ws)
		//        level.Info(logger).Log("EImp_Ws", response_chan.EImp_Ws, "EExp_Ws", response_chan.EExp_Ws)
		response_chan.P_W = a_chan.P_W - b_chan.P_W
		response_chan.Q_VAR = a_chan.Q_VAR - b_chan.Q_VAR
		response_chan.V_V = a_chan.V_V - b_chan.V_V
	}

	for ctIndex := 0; ctIndex < len(a.CTs); ctIndex++ {
		a_ct := a.CTs[ctIndex]
		b_ct := b.CTs[ctIndex]
		response_ct := &response.CTs[ctIndex]

		response_ct.CT = a_ct.CT
		response_ct.P_W = a_ct.P_W - b_ct.P_W
		response_ct.Q_VAR = a_ct.Q_VAR - b_ct.Q_VAR
		response_ct.V_V = a_ct.V_V - b_ct.V_V
	}

	//    level.Info(logger).Log("response", fmt.Sprintf("%v", response))

	return response
}

/**
 * Search local IPv4 addresses for Nerio devices
 */
func Discover(logger log.Logger) (devices []string) {
	ip, err := externalIP()
	if err != nil {
		level.Error(logger).Log("error", err)
	}
	level.Debug(logger).Log("localipaddress", ip)

	// drop the last byte of the ip address
	subIp := ip[:len(ip)-1]

	count := 255

	jobs := make(chan net.IP)
	results := make(chan net.IP, count)
	sem := make(semaphore.Semaphore, count)

	// spin up some worker threads
	workerCount := 64
	for w := 0; w < workerCount; w++ {
		go worker(logger, w, jobs, results, sem)
	}

	// dispatch the ip addresses to check to the channel
	// that the workers are listening on
	for i := 1; i <= count; i++ {
		ip := append(subIp, byte(i))
		level.Debug(logger).Log("ip", ip.String())
		newSlice := make([]byte, len(ip))
		copy(newSlice, ip)
		jobs <- newSlice
	}

	close(jobs)

	sem.Wait(count)

	// build the result slice
	select {
	case foundIP := <-results:
		devices = append(devices, foundIP.String())
	default:
		level.Info(logger).Log("zero", "devices")
	}

	return
}

func worker(logger log.Logger, id int, jobs <-chan net.IP, results chan<- net.IP, sem semaphore.Semaphore) {
	for ip := range jobs {
		level.Debug(logger).Log("id", id, "ip", ip)

		url := fmt.Sprintf("http://%s/current-sample", ip)
		success, _ := getUrl(url, logger)
		if success == true {
			results <- ip
		}

		sem.Signal()
	}

	level.Debug(logger).Log("id", id, "status", "shutting_down")
}

func getUrl(url string, logger log.Logger) (success bool, responseBody string) {
	var netClient = &http.Client{
		Timeout: time.Millisecond * 500,
	}

	level.Debug(logger).Log("url", url)

	response, err := netClient.Get(url)
	if err != nil {
		level.Debug(logger).Log("error", err)
		success = false
	} else {
		defer response.Body.Close()
		response_body, _ := ioutil.ReadAll(response.Body)
		responseBody = string(response_body)

		if response.StatusCode == 200 {
			success = true
		} else {
			success = false
		}
	}

	return
}

/* Return the ip address or an error
   Only support IPv4 */
func externalIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}

			return ip, nil
		}
	}

	return nil, errors.New("are you connected to the network?")
}
