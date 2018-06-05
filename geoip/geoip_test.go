package geoip

import (
	"testing"
)

func Test_GeoIPLookUp(t *testing.T) {
	err := Init("../templates/GeoLite2-City.mmdb")

	if err != nil {
		t.Fatal("GeoIPInit failed")
	}

	LookUp("35.194.178.199", "../templates/flags")
}
