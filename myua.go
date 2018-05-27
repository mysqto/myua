package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"log/syslog"
	"path"

	"github.com/mysqto/letshttps"
	"github.com/mysqto/myua/geoip"
	"gopkg.in/yaml.v2"
)

func getXRealIP(r *http.Request) (string, error) {
	var xRealIP string

	xRealIP = r.Header.Get("X-Real-IP")

	if len(xRealIP) == 0 {
		return "", fmt.Errorf("no \"X-Real-IP\" in request header")
	}

	ips := strings.Split(xRealIP, ",")

	for _, ip := range ips[:1] {
		clientIP := net.ParseIP(ip)

		if clientIP != nil {
			return clientIP.String(), nil
		}
		return "", fmt.Errorf("\"X-Real-IP\" in request header %s is not a valid ipaddress", ip)
	}

	return "", fmt.Errorf("empty \"X-Real-IP\" in request header")
}

func getXFFClient(r *http.Request) (string, error) {
	var xff string

	xff = r.Header.Get("X-Forwarded-For")
	if len(xff) == 0 {
		xff = r.Header.Get("X-Forwarded-For")
		if len(xff) == 0 {
			xff = r.Header.Get("X-FORWARDED-FOR") // matter
		}
	}

	if len(xff) == 0 {
		return "", fmt.Errorf("no \"X-Forwarded-For\" in request header")
	}

	ips := strings.Split(xff, ",")

	for _, ip := range ips[:1] {
		clientIP := net.ParseIP(ip)

		if clientIP != nil {
			return clientIP.String(), nil
		}
		return "", fmt.Errorf("\"X-Forwarded-For\" in request header %s is not a valid ipaddress", ip)
	}

	return "", fmt.Errorf("empty \"X-Forwarded-For\" in request header")
}

func getUserAgent(w http.ResponseWriter, r *http.Request) {

	uri := r.RequestURI
	ua := r.UserAgent()
	var ipAddr, country string
	var err error
	var data []byte
	var tpl *template.Template

	clientIP, err := getXFFClient(r)

	if err == nil {
		ipAddr = clientIP
	} else {
		ipAddr, _, err = net.SplitHostPort(r.RemoteAddr)

		if err != nil {
			ipAddr = ""
		}
	}

	if strings.HasPrefix(ua, "curl") ||
		strings.HasPrefix(ua, "Wget") ||
		strings.HasPrefix(ua, "HTTPie") ||
		strings.HasPrefix(ua, "fetch") {
		letshttps.SendHTTPResponse(w, []byte(ipAddr), http.StatusOK)
		return
	}

	if uri != "/" {
		path := yamlConfig.Templates + uri
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
		data, err := ioutil.ReadFile(path)

		if err != nil {
			log.Printf("error opening file %v : %v", path, err)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		}
		return
	}

	country, err = geoip.LookUp(ipAddr, yamlConfig.Templates+string(os.PathSeparator)+"flags")

	if err == nil {
		yamlConfig.Country = country
	}

	yamlConfig.IPAddr = ipAddr
	yamlConfig.UserAgent = ua

	data, err = ioutil.ReadFile("templates/index.html")

	if err != nil {
		return
	}

	tpl, err = template.New("index").Parse(string(data))

	tpl.Execute(w, yamlConfig)
}

func httpServer(addr string) {
	handlers := make(letshttps.HandleMap)
	handlers["/"] = getUserAgent
	log.Fatal(letshttps.NewHTTPServer(addr, handlers).ListenAndServe())
}

func httpsServer(certDir, domain, addr string) {
	handlers := make(letshttps.HandleMap)
	handlers["/"] = getUserAgent
	log.Fatal(letshttps.NewHTTPSServer(certDir, domain, addr, handlers).ListenAndServeTLS("", ""))
}

type config struct {
	Title       string   `yaml: "title"`
	Description string   `yaml: "description"`
	Author      string   `yaml: "author"`
	Keywords    []string `yaml: "keywords"`
	HTTPS       bool     `yaml: "https"`
	Domain      string   `yaml: "domain"`
	Cert        string   `yaml: "cert"`
	Templates   string   `yaml: "templates"`
	GeoIP       string   `yaml: "geoip"`
	QR          string   `yaml: "qr"`
	IPAddr      string
	Country     string
	UserAgent   string
}

func parseConfig() *config {

	cfg := config{}

	data, err := ioutil.ReadFile("templates/_config.yml")

	if err != nil {
		log.Fatalf("error reading _config.yml : %v", err)
	}

	err = yaml.Unmarshal([]byte(data), &cfg)

	if err != nil {
		log.Fatalf("error parsing _config.yml : %v", err)
	}

	data, err = ioutil.ReadFile(cfg.QR)

	if err != nil {
		log.Fatalf("error reading qr file %s : %v", cfg.QR, err)
	}

	cfg.QR = base64.RawStdEncoding.EncodeToString(data)

	return &cfg
}

// InitLog init sys log
func initLog(level syslog.Priority) {

	logWriter, err := syslog.New(level, path.Base(os.Args[0]))

	if err == nil {
		log.SetOutput(logWriter)
	}
}

var yamlConfig *config

func main() {

	initLog(syslog.LOG_NOTICE)

	yamlConfig = parseConfig()

	geoip.Init(yamlConfig.GeoIP)

	finish := make(chan bool)

	if yamlConfig.HTTPS {
		go httpsServer("cert", yamlConfig.Domain, ":https")
	} else {
		go httpServer(":8080")
	}

	<-finish
}
