package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	flagDevice = flag.String("d", "", "Sonos device name (required)")
	flagAppend = flag.Bool("a", false, "Append to device queue")
)

func main() {
	flag.Parse()

	if *flagDevice == "" {
		log.Fatal("You must use '-d <device>' to specify a Sonos device")
	}

	found, err := Search("urn:schemas-upnp-org:device:ZonePlayer:1")
	if err != nil {
		log.Fatalf("Search: %s", err)
	}

	if len(found) == 0 {
		log.Fatal("No Sonos devices found")
	}

	var deviceURL *url.URL
	for _, f := range found {
		u, err := url.Parse(f.Get("Location"))
		if err != nil {
			log.Printf("Parsing %s: %s", f.Get("Location"), err)
			continue
		}

		d, err := fetchDevice(u)
		if err != nil {
			log.Printf("Fetching %s: %s", u.String(), err)
			continue
		}

		if d.RoomName == *flagDevice {
			deviceURL = u
		}
	}

	if deviceURL == nil {
		log.Fatalf("Device not found: %s", *flagDevice)
	}

	if !*flagAppend {
		if err := removeAllTracksFromQueue(deviceURL); err != nil {
			log.Fatalf("Clearing queue: %s: %s", *flagDevice, err)
		}
	}

	for _, item := range flag.Args() {
		if _, err := url.Parse(item); err != nil {
			log.Printf("Skipping non-url: %s", item)
			continue
		}

		if err := addURIToQueue(deviceURL, item); err != nil {
			log.Fatalf("Enqueueing %s: %s", item, err)
		}
	}
}

// Search performs an SSDP query via multicast.
func Search(query string) ([]http.Header, error) {
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := strings.Join([]string{
		"M-SEARCH * HTTP/1.1",
		"HOST: 239.255.255.250:1900",
		"MAN: \"ssdp:discover\"",
		"ST: " + query,
		"MX: 1",
	}, "\r\n")

	addr, err := net.ResolveUDPAddr("udp", "239.255.255.250:1900")
	if err != nil {
		return nil, err
	}

	_, err = conn.WriteTo([]byte(req), addr)
	if err != nil {
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(2 * time.Second))

	var devices []http.Header
	for {
		buf := make([]byte, 65536)

		n, _, err := conn.ReadFrom(buf)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			break
		} else if err != nil {
			log.Printf("ReadFrom error: %s", err)
			break
		}

		r := bufio.NewReader(bytes.NewReader(buf[:n]))

		resp, err := http.ReadResponse(r, &http.Request{})
		if err != nil {
			log.Printf("ReadResponse error: %s", err)
		}
		resp.Body.Close()

		for _, head := range resp.Header["St"] {
			if head == query {
				devices = append(devices, resp.Header)
				break
			}
		}
	}

	return devices, nil
}

func fetchDevice(u *url.URL) (*device, error) {
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var root struct {
		Device device `xml:"device"`
	}
	if err = xml.NewDecoder(resp.Body).Decode(&root); err != nil {
		log.Printf("Decode %s: %s", u.String(), err)
	}

	return &root.Device, err
}

type device struct {
	RoomName string `xml:"roomName"`
}

func removeAllTracksFromQueue(base *url.URL) error {
	u := *base
	u.Path = "/MediaRenderer/AVTransport/Control"

	const body = `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:RemoveAllTracksFromQueue xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:RemoveAllTracksFromQueue></s:Body></s:Envelope>`

	req, err := http.NewRequest("POST", u.String(), bytes.NewReader([]byte(body)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")

	// Setting SOAPACTION directly in the Header map to preserve
	// non-normalized case.
	req.Header["SOAPACTION"] = []string{"urn:schemas-upnp-org:service:AVTransport:1#RemoveAllTracksFromQueue"}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %d", u.Path, resp.StatusCode)
	}

	return nil
}

func addURIToQueue(base *url.URL, item string) error {
	u := *base
	u.Path = "/MediaRenderer/AVTransport/Control"

	const bodyFmt = `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:AddURIToQueue xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><EnqueuedURI>%s</EnqueuedURI><EnqueuedURIMetaData></EnqueuedURIMetaData><DesiredFirstTrackNumberEnqueued>0</DesiredFirstTrackNumberEnqueued><EnqueueAsNext>0</EnqueueAsNext></u:AddURIToQueue></s:Body></s:Envelope>`

	body := fmt.Sprintf(bodyFmt, html.EscapeString(item))

	req, err := http.NewRequest("POST", u.String(), bytes.NewReader([]byte(body)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")

	// Setting SOAPACTION directly in the Header map to preserve
	// non-normalized case.
	req.Header["SOAPACTION"] = []string{"urn:schemas-upnp-org:service:AVTransport:1#AddURIToQueue"}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %d", u.Path, resp.StatusCode)
	}

	return nil
}
