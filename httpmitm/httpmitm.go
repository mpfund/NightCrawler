package httpmitm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// A very simple http proxy

const (
	rsaBits    = 2048
	certFolder = "cert"
)

var (
	mu sync.Mutex
)

type CallbackRequest func(*http.Request)
type CallbackResponse func(*http.Request, *http.Response, time.Duration)

/*
func main() {
	simpleProxyHandler := http.HandlerFunc(simpleProxyHandlerFunc)
	http.ListenAndServe(":8080", simpleProxyHandler)
	//createCerts("test")
}*/

func logRequest(r *http.Request, add string) {
	fmt.Println(add)
	fmt.Println("Host", r.Host)
	fmt.Println("Url", r.URL.String())
	fmt.Println("Mehtod", r.Method)
	fmt.Println("--------------")
}

func buildHttpsUrl(r *http.Request) *url.URL {
	url, err := url.Parse("https://" + r.Host + r.URL.String())
	if err != nil {
		fmt.Println("error", err.Error())
	}
	return url
}

func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func getCertPath(hostname string) (string, string) {
	cert := path.Join(certFolder, hostname+".pem")
	key := path.Join(certFolder, hostname+".key")
	return cert, key
}

func handleResponse(resp *http.Response) {
	//fmt.Println(resp.Proto)
	raw, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	buf1 := bytes.NewBuffer(raw)
	bufReader := ioutil.NopCloser(buf1)
	resp.Body = bufReader

	buf2 := bytes.NewBuffer(raw)

	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, _ = gzip.NewReader(buf2)
		defer reader.Close()
	default:
		reader = ioutil.NopCloser(buf2)
	}

	io.Copy(os.Stdout, reader)
}

func GenSimpleHandlerFunc(cReq CallbackRequest, cResp CallbackResponse) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		simpleProxyHandlerFunc(w, r, cReq, cResp)
	}
}

func simpleProxyHandlerFunc(w http.ResponseWriter, r *http.Request, cReq CallbackRequest, cResp CallbackResponse) {
	if r.Method == "CONNECT" {
		hostname := strings.Split(r.Host, ":")[0]
		fmt.Println("cert for ", hostname)

		certFile, keyFile := getCertPath(hostname)

		if !pathExists(certFile) {
			log.Println("creating cert for", hostname)
			createCerts(hostname)
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		checkError(err)

		config := tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		}

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
			return
		}
		conn, buff, err := hj.Hijack()
		buff.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		//fmt.Println("sending ok ", r.URL.String())
		buff.Flush()

		go (func() {

			if err != nil {
				return
			}
			defer conn.Close()
			tlsCon := tls.Server(conn, &config)
			clientTlsReader := bufio.NewReader(tlsCon)
			clientTlsWriter := bufio.NewWriter(tlsCon)
			tlsCon.Handshake()

			for {
				r, err := http.ReadRequest(clientTlsReader)
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				//httpc := &http.Client{}

				r.RequestURI = ""
				r.URL = buildHttpsUrl(r)

				//logRequest(r, "https")
				resp, err := http.DefaultTransport.RoundTrip(r)

				if err != nil {
					fmt.Println("https http client error ", err.Error())
					continue
				}
				//fmt.Println(resp)
				handleResponse(resp)
				resp.Write(clientTlsWriter)
				clientTlsWriter.Flush()
			}
		})()
	} else {
		cReq(r)
		timeStart := time.Now()
		httpc := &http.Client{}
		// request uri can't be set in client requests
		r.RequestURI = ""
		resp, err := httpc.Do(r)

		if err != nil {
			logRequest(r, "http, error:"+err.Error())
			fmt.Println(err.Error())
		}

		copyHeaders(w.Header(), resp.Header)
		// copy content
		//defer resp.Body.Close()
		bodyContent, _ := ioutil.ReadAll(resp.Body)
		w.Write(bodyContent)

		//restore body
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyContent))
		cResp(r, resp, time.Now().Sub(timeStart))
	}
}

func copyHeaders(dest http.Header, source http.Header) {
	for header := range source {
		dest.Add(header, source.Get(header))
	}
}

func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func createCerts(hostName string) {
	mu.Lock()
	defer mu.Unlock()

	caCertFile := "c:\\sslkeys\\server2.cert"
	caKeyFile := "c:\\sslkeys\\server2.key"

	certFile, keyFile := getCertPath(hostName)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(mrand.Int63n(time.Now().Unix())),
		Subject: pkix.Name{
			Country:            []string{"DE"},
			Organization:       []string{"PPP"},
			OrganizationalUnit: []string{"MMM"},
			CommonName:         hostName,
		},

		NotBefore:    time.Now().UTC(),
		NotAfter:     time.Now().AddDate(10, 0, 0).UTC(),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		DNSNames:     []string{hostName},
		//PermittedDNSDomains: []string{name},
	}

	if ip := net.ParseIP(hostName); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, hostName)
	}

	var err error = nil
	var rootCA tls.Certificate

	rootCA, err = tls.LoadX509KeyPair(caCertFile, caKeyFile)
	checkError(err)

	rootCA.Leaf, err = x509.ParseCertificate(rootCA.Certificate[0])
	checkError(err)

	var priv *rsa.PrivateKey

	priv, err = rsa.GenerateKey(rand.Reader, rsaBits)
	checkError(err)

	var derBytes []byte

	derBytes, err = x509.CreateCertificate(rand.Reader, template, rootCA.Leaf, &priv.PublicKey, rootCA.PrivateKey)
	checkError(err)

	if !pathExists(certFolder) {
		os.Mkdir(certFolder, 0777)
	}

	certOut, err := os.Create(certFile)
	checkError(err)

	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	checkError(err)

	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

}
