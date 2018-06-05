package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"encoding/base64"
	"log/syslog"
	"path"

	"github.com/mysqto/letshttps"
	"github.com/mysqto/myua/geoip"
	"github.com/mysqto/onsale/utils"
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
		utils.SendHTTPResponse(w, ipAddr, http.StatusOK)
		return
	}

	if uri != "/" {
		path := yamlConfig.Site.Base + uri
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

	htmlTpl := makeTemplate()
	country, err = geoip.LookUp(ipAddr, yamlConfig.Site.Base+string(os.PathSeparator)+"flags")

	if err == nil {
		htmlTpl.Country = country
	}

	htmlTpl.IPAddr = ipAddr
	htmlTpl.UserAgent = ua
	htmlTpl.Host = letshttps.StripHost(r.Host)

	data, err = ioutil.ReadFile("templates/index.html")

	if err != nil {
		return
	}

	tpl, err = template.New("index").Parse(string(data))

	tpl.Execute(w, htmlTpl)
}

func serve(https bool, serverConfig *letshttps.ServerConfig) {
	if https {
		log.Fatal(letshttps.NewHTTPSServer(serverConfig).ListenAndServeTLS("", ""))
	} else {
		log.Fatal(letshttps.NewHTTPServer(serverConfig).ListenAndServe())
	}
}

func makeTemplate() *htmlTemplate {
	tpl := htmlTemplate(yamlConfig.Site.Config)
	return &tpl
}

type htmlTemplate struct {
	Title       string   `yaml: "title"`
	Description string   `yaml: "description"`
	Author      string   `yaml: "author"`
	Keywords    []string `yaml: "keywords"`
	Domain      []string `yaml: "domain"`
	GeoIP       string   `yaml: "geoip"`
	GAid        string   `yaml: "gaid"`
	QR          string   `yaml: "qr"`
	Host        string
	IPAddr      string
	UserAgent   string
	Country     string
}

type config struct {
	Site struct {
		Base    string `yaml: "base"`
		Config htmlTemplate
		HTTPS struct {
			Enabled bool   `yaml: "enabled"`
			Only    bool   `yaml: "only"`
			Port    string `yaml: "port"`
			Cert    string `yaml: "cert"`
		} `yaml: "https"`
		HTTP struct {
			Port string `yaml: "port"`
		} `yaml: "http"`
	}
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

	data, err = ioutil.ReadFile(cfg.Site.Config.QR)

	if err != nil {
		log.Fatalf("error reading qr file %s : %v", cfg.Site.Config.QR, err)
	}

	cfg.Site.Config.QR = base64.StdEncoding.EncodeToString(data)

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

	geoip.Init(yamlConfig.Site.Config.GeoIP)

	finish := make(chan bool)

	handlers := make(letshttps.HandleMap)
	handlers["/"] = getUserAgent

	cfg := &letshttps.ServerConfig{
		Redirect:  yamlConfig.Site.HTTPS.Only,
		HTTPAddr:  letshttps.JoinAddr(yamlConfig.Site.HTTP.Port),
		HTTPSAddr: letshttps.JoinAddr(yamlConfig.Site.HTTPS.Port),
		Domains:   yamlConfig.Site.Config.Domain,
		CertDir:   yamlConfig.Site.HTTPS.Cert,
		Handlers:  handlers,
		Timeout:   5,
	}

	go serve(yamlConfig.Site.HTTPS.Enabled, cfg)

	<-finish
}
