/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

/* Copyright 2017 Chris Morgan <chmorgan@gmail.com> */

package neurio

import (
	"encoding/json"
	"fmt"
	"testing"

	"go.uber.org/zap"
)

var testJSON = "{\"sensorId\":\"0x0000C47F51019E05\",\"timestamp\":\"2017-06-26T00:30:26Z\",\"channels\":[{\"type\":\"PHASE_A_CONSUMPTION\",\"ch\":1,\"eImp_Ws\":21837827195,\"eExp_Ws\":112135409,\"p_W\":1121,\"q_VAR\":15,\"v_V\":123.574},{\"type\":\"PHASE_B_CONSUMPTION\",\"ch\":2,\"eImp_Ws\":20624834816,\"eExp_Ws\":123249487,\"p_W\":1428,\"q_VAR\":314,\"v_V\":123.729},{\"type\":\"NET\",\"ch\":3,\"eImp_Ws\":42045777679,\"eExp_Ws\":1289626261,\"p_W\":2555,\"q_VAR\":293,\"v_V\":123.655},{\"type\":\"GENERATION\",\"ch\":4,\"eImp_Ws\":1497258783,\"eExp_Ws\":6754561,\"p_W\":-6,\"q_VAR\":36,\"v_V\":123.648},{\"type\":\"CONSUMPTION\",\"ch\":5,\"eImp_Ws\":1361713487,\"eExp_Ws\":199739472,\"p_W\":2549,\"q_VAR\":329,\"v_V\":123.652}],\"cts\":[{\"ct\":1,\"p_W\":1127,\"q_VAR\":-3,\"v_V\":123.576},{\"ct\":2,\"p_W\":1428,\"q_VAR\":296,\"v_V\":123.735},{\"ct\":3,\"p_W\":1,\"q_VAR\":18,\"v_V\":123.723},{\"ct\":4,\"p_W\":-6,\"q_VAR\":17,\"v_V\":123.573}]}"
var testJSON2 = "{\"sensorId\":\"0x0000C47F51019E05\",\"timestamp\":\"2017-06-26T00:30:29Z\",\"channels\":[{\"type\":\"PHASE_A_CONSUMPTION\",\"ch\":1,\"eImp_Ws\":21837827395,\"eExp_Ws\":112135509,\"p_W\":1121,\"q_VAR\":15,\"v_V\":123.574},{\"type\":\"PHASE_B_CONSUMPTION\",\"ch\":2,\"eImp_Ws\":20624834916,\"eExp_Ws\":123249587,\"p_W\":1428,\"q_VAR\":314,\"v_V\":123.729},{\"type\":\"NET\",\"ch\":3,\"eImp_Ws\":42045777679,\"eExp_Ws\":1289626261,\"p_W\":2555,\"q_VAR\":293,\"v_V\":123.655},{\"type\":\"GENERATION\",\"ch\":4,\"eImp_Ws\":1497258783,\"eExp_Ws\":6754561,\"p_W\":-6,\"q_VAR\":36,\"v_V\":123.648},{\"type\":\"CONSUMPTION\",\"ch\":5,\"eImp_Ws\":1361713487,\"eExp_Ws\":199739472,\"p_W\":2549,\"q_VAR\":329,\"v_V\":123.652}],\"cts\":[{\"ct\":1,\"p_W\":1127,\"q_VAR\":-3,\"v_V\":123.576},{\"ct\":2,\"p_W\":1428,\"q_VAR\":296,\"v_V\":123.735},{\"ct\":3,\"p_W\":1,\"q_VAR\":18,\"v_V\":123.723},{\"ct\":4,\"p_W\":-6,\"q_VAR\":17,\"v_V\":123.573}]}"

func TestCalculateDelta(t *testing.T) {
	logger := zap.NewExample().Sugar()

	samples := make([]CurrentSampleResponse, 0)

	var sampleResponse *CurrentSampleResponse = &CurrentSampleResponse{}
	err := json.Unmarshal([]byte(testJSON), sampleResponse)
	if err != nil {
		fmt.Print(err)
	}

	// prepend the new sample
	samples = append(samples, CurrentSampleResponse{})
	copy(samples[1:], samples)
	samples[0] = *sampleResponse

	var sampleResponse2 *CurrentSampleResponse = &CurrentSampleResponse{}
	err = json.Unmarshal([]byte(testJSON2), sampleResponse2)
	if err != nil {
		fmt.Print(err)
	}

	// prepend the new sample
	samples = append(samples, CurrentSampleResponse{})
	copy(samples[1:], samples)
	samples[0] = *sampleResponse2

	// confirm sample count
	expectedSampleLength := 2
	if len(samples) != expectedSampleLength {
		t.Errorf("expected len(samples) of %d but was %d\n", expectedSampleLength, len(samples))
	}

	fmt.Printf("samples[0]:\n%+v\n", samples[0])
	fmt.Printf("samples[1]:\n%+v\n", samples[1])

	nearestSample, actualDuration, err := FindNearestSampleForDuration("5s", samples, logger)

	// NOTE: samples[0] is the newest sample
	delta := DeltaSample(samples[0], nearestSample, logger)

	deltaString := fmt.Sprintf("%#v", delta)
	logger.Infow("delta entry", "delta", deltaString, "actualDuration", actualDuration)

	expected_EImp_Ws := int64(200)
	if delta.Channels[0].EImp_Ws != expected_EImp_Ws {
		t.Errorf("Expected EImp_Ws to be %d but is %d\n", expected_EImp_Ws, delta.Channels[0].EImp_Ws)
		fmt.Printf("delta:\n%+v\n", delta)
	}

	expected_EExp_Ws := int64(100)
	if delta.Channels[0].EExp_Ws != expected_EExp_Ws {
		t.Errorf("Expected EExp_Ws to be %d but is %d\n", expected_EExp_Ws, delta.Channels[0].EExp_Ws)
		fmt.Printf("%+v", delta)
	}
}
