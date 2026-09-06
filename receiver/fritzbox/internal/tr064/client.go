// Package tr064 implements a minimal TR-064 (UPnP/SOAP) client for
// AVM Fritz!Box devices. It supports service discovery via the device
// description document and SOAP action calls with optional HTTP digest
// authentication as required by Fritz!Box devices.
package tr064

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Service identifies a TR-064 service, e.g. "urn:dslforum-org:service:DeviceInfo:1".
type Service struct {
	// Type is the full UPnP service type, e.g. "urn:dslforum-org:service:DeviceInfo:1".
	Type string
	// ControlURL is the path SOAP actions for this service are posted to,
	// e.g. "/upnp/control/deviceinfo".
	ControlURL string
}

// Error is a SOAP fault returned by the device.
type Error struct {
	// Code is the UPnP error code, e.g. 401 for "Invalid Action".
	Code int
	// Description is the human-readable UPnP error description.
	Description string
}

func (e *Error) Error() string {
	return fmt.Sprintf("tr064: UPnP error %d: %s", e.Code, e.Description)
}

// Client is a TR-064 client for a single Fritz!Box device.
type Client struct {
	endpoint   string
	httpClient *http.Client
	username   string
	password   string
}

// NewClient creates a TR-064 client for the device at endpoint
// (e.g. "http://fritz.box:49000"). Username and password may be empty;
// actions that require authentication will then fail with an
// authentication error on call.
func NewClient(endpoint, username, password string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("tr064: invalid endpoint %q: %w", endpoint, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("tr064: endpoint %q must be an absolute URL", endpoint)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		httpClient: httpClient,
		username:   username,
		password:   password,
	}, nil
}

// deviceDescription is the subset of tr64desc.xml we care about.
type deviceDescription struct {
	XMLName xml.Name `xml:"root"`
	Device  device   `xml:"device"`
}

type device struct {
	Services []service `xml:"serviceList>service"`
	Devices  []device  `xml:"deviceList>device"`
}

type service struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

// Services fetches the device description document and returns all services
// the device offers, including services of nested sub-devices.
func (c *Client) Services(ctx context.Context) ([]Service, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/tr64desc.xml", nil)
	if err != nil {
		return nil, fmt.Errorf("tr064: building discovery request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tr064: fetching device description: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tr064: fetching device description: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("tr064: reading device description: %w", err)
	}
	var desc deviceDescription
	if err := xml.Unmarshal(body, &desc); err != nil {
		return nil, fmt.Errorf("tr064: parsing device description: %w", err)
	}
	var services []Service
	collectServices(desc.Device, &services)
	return services, nil
}

func collectServices(d device, out *[]Service) {
	for _, s := range d.Services {
		*out = append(*out, Service{Type: s.ServiceType, ControlURL: s.ControlURL})
	}
	for _, sub := range d.Devices {
		collectServices(sub, out)
	}
}

// soapEnvelope is the request envelope. The action element is rendered
// manually so that optional input arguments can be nested inside it.
type soapEnvelope struct {
	XMLName xml.Name `xml:"s:Envelope"`
	XmlnsS  string   `xml:"xmlns:s,attr"`
	EncSt   string   `xml:"s:encodingStyle,attr"`
	Body    soapBody
}

type soapBody struct {
	XMLName xml.Name `xml:"s:Body"`
	Action  soapAction
}

type soapAction struct {
	XMLName xml.Name
	XmlnsU  string `xml:"xmlns:u,attr"`
	Args    []soapArg
}

type soapArg struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// soapFault models the fault response returned by the device.
type soapFaultEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Fault *struct {
			Detail struct {
				UPnPError struct {
					ErrorCode        int    `xml:"errorCode"`
					ErrorDescription string `xml:"errorDescription"`
				} `xml:"UPnPError"`
			} `xml:"detail"`
		} `xml:"Fault"`
	} `xml:"Body"`
}

// Call invokes a TR-064 action without input arguments. It is equivalent to
// CallWithArgs with a nil args map.
func (c *Client) Call(ctx context.Context, serviceType, controlURL, action string) (map[string]string, error) {
	return c.CallWithArgs(ctx, serviceType, controlURL, action, nil)
}

// CallWithArgs invokes a TR-064 action with optional input arguments and
// returns the response out arguments as a map. If the device responds with a
// SOAP fault, the returned error is an *Error.
func (c *Client) CallWithArgs(ctx context.Context, serviceType, controlURL, action string, args map[string]string) (map[string]string, error) {
	envelopeArgs := make([]soapArg, 0, len(args))
	for name, value := range args {
		envelopeArgs = append(envelopeArgs, soapArg{
			XMLName: xml.Name{Local: name},
			Value:   value,
		})
	}
	env := soapEnvelope{
		XmlnsS: "http://schemas.xmlsoap.org/soap/envelope/",
		EncSt:  "http://schemas.xmlsoap.org/soap/encoding/",
		Body: soapBody{
			Action: soapAction{
				XMLName: xml.Name{Local: "u:" + action},
				XmlnsU:  serviceType,
				Args:    envelopeArgs,
			},
		},
	}
	payload, err := xml.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("tr064: marshaling SOAP request: %w", err)
	}

	callURL := c.endpoint + controlURL
	soapAction := serviceType + "#" + action

	resp, err := c.doCall(ctx, callURL, soapAction, payload, "")
	if err != nil {
		return nil, err
	}

	// On 401 with a digest challenge and configured credentials, retry once
	// with the Authorization header.
	if resp.StatusCode == http.StatusUnauthorized && c.username != "" {
		challenge := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		auth, err := digestAuthorization(challenge, "POST", callURL, c.username, c.password)
		if err != nil {
			return nil, fmt.Errorf("tr064: action %s: digest authentication failed: %w", action, err)
		}
		resp, err = c.doCall(ctx, callURL, soapAction, payload, auth)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("tr064: action %s: reading response: %w", action, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Code: http.StatusUnauthorized, Description: "authentication required"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tr064: action %s: unexpected status %s", action, resp.Status)
	}

	// Distinguish fault from success: faults contain a Fault element.
	if bytes.Contains(body, []byte(":Fault>")) {
		var fault soapFaultEnvelope
		if err := xml.Unmarshal(body, &fault); err != nil {
			return nil, fmt.Errorf("tr064: action %s: parsing SOAP fault: %w", action, err)
		}
		if fault.Body.Fault != nil {
			ue := fault.Body.Fault.Detail.UPnPError
			return nil, &Error{Code: ue.ErrorCode, Description: ue.ErrorDescription}
		}
		return nil, fmt.Errorf("tr064: action %s: malformed SOAP fault", action)
	}

	return parseActionResponse(body, action)
}

func (c *Client) doCall(ctx context.Context, callURL, soapAction string, payload []byte, authorization string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("tr064: building SOAP request: %w", err)
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", soapAction)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tr064: SOAP call %s: %w", soapAction, err)
	}
	return resp, nil
}

// parseActionResponse extracts the out arguments from a successful action
// response: <u:ActionResponse><NewFoo>bar</NewFoo>...</u:ActionResponse>.
func parseActionResponse(body []byte, action string) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("tr064: action %s: parsing response: %w", action, err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != action+"Response" {
			continue
		}
		out := map[string]string{}
		for {
			tok, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("tr064: action %s: parsing response arguments: %w", action, err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				var val string
				if err := decoder.DecodeElement(&val, &t); err != nil {
					return nil, fmt.Errorf("tr064: action %s: decoding argument %s: %w", action, t.Name.Local, err)
				}
				out[t.Name.Local] = val
			case xml.EndElement:
				if t.Name.Local == action+"Response" {
					return out, nil
				}
			}
		}
	}
}
