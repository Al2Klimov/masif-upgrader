package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/Al2Klimov/masif-upgrader/common"
	"io/ioutil"
	"net/http"
)

type badHttpStatus struct {
	status int
}

func (self *badHttpStatus) Error() string {
	return fmt.Sprintf("bad HTTP response status %d (expected 200)", self.status)
}

type api struct {
	baseUrl string
	client  *http.Client
}

func newApi(master string, tlsCfg struct{ cert, key, ca string }) (result *api, err error) {
	clientCert, errLXKP := tls.LoadX509KeyPair(tlsCfg.cert, tlsCfg.key)
	if errLXKP != nil {
		return nil, errLXKP
	}

	rootCA, errRF := ioutil.ReadFile(tlsCfg.ca)
	if errRF != nil {
		return nil, errRF
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(rootCA)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      rootCAs,
				CipherSuites: common.ApiTlsCipherSuites,
				MinVersion:   common.ApiTlsMinVersion,
			},
		},
	}

	return &api{baseUrl: "https://" + master + "/v1", client: client}, nil
}

func (self *api) reportTasks(tasks map[common.PkgMgrTask]struct{}) (approvedTasks map[common.PkgMgrTask]struct{}, err error) {
	jsn, errPMT2A := common.PkgMgrTasks2Api(tasks)
	if errPMT2A != nil {
		return nil, errPMT2A
	}

	res, errPost := self.client.Post(
		self.baseUrl+"/pending-tasks",
		"application/json",
		bytes.NewBuffer(jsn),
	)
	if errPost != nil {
		return nil, errPost
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, &badHttpStatus{status: res.StatusCode}
	}

	body, errRA := ioutil.ReadAll(res.Body)
	if errRA != nil {
		return nil, errRA
	}

	return common.Api2PkgMgrTasks(body)
}
