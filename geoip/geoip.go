package geoip

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"fmt"

	"github.com/oschwald/geoip2-golang"
)

var db *geoip2.Reader

// Init init GeoIP database in memory
func Init(file string) error {
	var err error
	if db == nil {
		db, err = geoip2.Open(file)
		if err != nil {
			log.Printf("error opening GeoIP database: %v", err)
		}
	}
	return nil
}

// LookUp init GeoIP database in memory
func LookUp(addr, flags string) (string, error) {
	var err error
	var ip net.IP
	var data []byte
	var city *geoip2.City

	ip = net.ParseIP(addr)

	city, err = db.City(ip)

	if err != nil {
		log.Printf("error looking up GeoIP database: %v", err)
		return "", err
	}

	countryCode := city.Country.IsoCode

	if len(countryCode) == 0 {
		countryCode = city.RegisteredCountry.IsoCode
		if len(countryCode) == 0 {
			return "", fmt.Errorf("caonnot find country for IP Address : %s", addr)
		}
	}

	log.Printf("countryCode = %s", countryCode)

	path := flags + string(os.PathSeparator) + strings.ToLower(countryCode) + ".svg"

	data, err = ioutil.ReadFile(path)

	if err != nil {
		log.Printf("error opening file %s : %v", path, err)
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
