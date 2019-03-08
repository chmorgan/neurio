[![Go Report Card](https://goreportcard.com/badge/github.com/chmorgan/neurio)](https://goreportcard.com/report/github.com/chmorgan/neurio)

# neurio

Go library for discovering and processing data from Neurio devices.

Developed for local device access, eg. you are running the library on the same network
where your neurio device is located. This greatly simplifies the usage of the device, there
are no accounts, keys, secrets, oauth2 crap to deal with etc.

Direct access also means that if neurio were to go out of business, and I certainly hope they
don't as their energy monitor works great and is about the only one that provides a direct
api like this, we'll all be able to continue to use our devices.

Chris Morgan <chmorgan@gmail.com>
