package forwarding

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"

	"github.com/paultyng/go-unifi/unifi"
)

type UnifiClient struct {
	site  string
	inner *unifi.Client
}

func NewUnifiClient(site string, baseURL string, user string, pass string, insecure bool) (UnifiClient, error) {
	var err error

	ctx := context.TODO()
	c := unifi.Client{}
	if insecure {
		jar, _ := cookiejar.New(nil)
		err = c.SetHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Jar: jar,
		})
		if err != nil {
			return UnifiClient{}, err
		}
	}
	err = c.SetBaseURL(baseURL)
	if err != nil {
		return UnifiClient{}, err
	}
	err = c.Login(ctx, user, pass)
	if err != nil {
		return UnifiClient{}, err
	}
	client := UnifiClient{
		site:  site,
		inner: &c,
	}

	return client, nil
}

func (c UnifiClient) CreatePortForwards(ctx context.Context, forwards []PortForward) error {
	for _, forward := range forwards {
		_, err := c.inner.CreatePortForward(ctx, c.site, &unifi.PortForward{
			Enabled:       true,
			Name:          forward.Name,
			Fwd:           forward.Address,
			FwdPort:       fmt.Sprint(forward.Port),
			DstPort:       fmt.Sprint(forward.Port), // TODO: support multiple hostPorts targeting the same port
			Src:           "any",
			Proto:         "tcp_udp", // tcp, udp, or tcp_udp
			PfwdInterface: "wan",     // wan, wan2, or both
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c UnifiClient) ListPortForwards(ctx context.Context) ([]PortForward, error) {
	forwards, err := c.inner.ListPortForward(ctx, c.site)
	if err != nil {
		return nil, err
	}

	convertedForwards := []PortForward{}
	for _, forward := range forwards {
		// TODO: fwdport can have value like 8000-8001!
		// Right now, we skip them since those aren't related to the controller
		if strings.Contains(forward.FwdPort, "-") {
			continue
		}
		port, err := strconv.Atoi(forward.FwdPort)
		if err != nil {
			return nil, err
		}

		convertedForwards = append(convertedForwards, PortForward{
			Name:    forward.Name,
			Address: forward.Fwd,
			Port:    int32(port),
		})
	}
	return convertedForwards, nil
}

func (c UnifiClient) DeletePortForwards(ctx context.Context, forwards []PortForward) error {
	// hack to fetch the ids of the to-be-deleted forwarding rules as we do not track
	// the ids in the controller as it is Unifi-specific
	existingForwards, err := c.inner.ListPortForward(ctx, c.site)
	if err != nil {
		return err
	}
	var idsToDelete []string
	for _, forward := range forwards {
		for _, existingForward := range existingForwards {
			if forward.Name == existingForward.Name {
				idsToDelete = append(idsToDelete, existingForward.ID)
			}
		}
	}

	for _, id := range idsToDelete {
		err = c.inner.DeletePortForward(ctx, c.site, id)
		if err != nil {
			return err
		}
	}
	return nil
}
