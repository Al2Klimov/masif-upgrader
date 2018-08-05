package main

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"github.com/Al2Klimov/masif-upgrader/common"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

func newApi(listen string, tlsCfg struct{ cert, key, ca, crl string }) (result *http.Server, err error) {
	cert, errLXKP := tls.LoadX509KeyPair(tlsCfg.cert, tlsCfg.key)
	if errLXKP != nil {
		return nil, errLXKP
	}

	rootCA, errRF := ioutil.ReadFile(tlsCfg.ca)
	if errRF != nil {
		return nil, errRF
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(rootCA)

	var crlValidator func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error = nil
	if tlsCfg.crl != "" {
		crlValidator = apiMkCrlValidator(tlsCfg.crl)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/pending-tasks", apiV1PendingTasks)
	mux.HandleFunc("/", apiDefault)

	return &http.Server{
		Addr:    listen,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates:             []tls.Certificate{cert},
			VerifyPeerCertificate:    crlValidator,
			ClientAuth:               tls.RequireAndVerifyClientCert,
			ClientCAs:                rootCAs,
			CipherSuites:             common.ApiTlsCipherSuites,
			PreferServerCipherSuites: true,
			MinVersion:               common.ApiTlsMinVersion,
		},
	}, nil
}

var apiRevoked = errors.New("all valid certificates directly below root have been revoked")

func apiMkCrlValidator(crlPath string) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	mutex := sync.RWMutex{}
	var timesUpdated uint64 = 0
	var crl *pkix.CertificateList = nil
	var revokedCerts map[string]struct{} = nil

	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		mutex.RLock()

		if crl == nil || crl.HasExpired(time.Now()) {
			timesUpdatedLastSeen := timesUpdated

			mutex.RUnlock()
			mutex.Lock()

			if timesUpdated == timesUpdatedLastSeen {
				timesUpdated++

				rawCRL, errRF := ioutil.ReadFile(crlPath)
				if errRF != nil {
					mutex.Unlock()
					return errRF
				}

				freshCRL, errPCRL := x509.ParseCRL(rawCRL)
				if errPCRL != nil {
					mutex.Unlock()
					return errPCRL
				}

				crl = freshCRL

				revokedCerts = map[string]struct{}{}
				for _, revokedCert := range crl.TBSCertList.RevokedCertificates {
					revokedCerts[revokedCert.SerialNumber.Text(62)] = struct{}{}
				}
			}

			mutex.Unlock()
			mutex.RLock()
		}

		defer mutex.RUnlock()

		for _, chains := range verifiedChains {
			if chains[len(chains)-1].CheckCRLSignature(crl) == nil {
				if _, isRevoked := revokedCerts[chains[len(chains)-2].SerialNumber.Text(62)]; !isRevoked {
					return nil
				}
			}
		}

		return apiRevoked
	}
}

func apiV1PendingTasks(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, errRA := ioutil.ReadAll(request.Body)
	if errRA != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, errA2PMT := common.Api2PkgMgrTasks(body)
	if errA2PMT != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: something useful
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("[]"))
}

func apiDefault(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotFound)
}
