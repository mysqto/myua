package geoip

import (
	"log"
	"testing"
)

func Test_GeoIPLookUp(t *testing.T) {
	err := Init("../templates/GeoLite2-City.mmdb")

	if err != nil {
		t.Fatal("GeoIPInit failed")
	}

	svg, err := LookUp("200.5.12.0", "../templates/flags")

	if err != nil {
		t.Fatal(err)
	}

	log.Println(svg)
}
