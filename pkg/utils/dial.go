package utils

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/config"
)

func Dial(dial *config.Dial, index int) (net.Conn, error) {
	if len(dial.Addrs) == 0 {
		return nil, errors.New("illegal addrs")
	}
	var (
		network string = dial.Network
		addr    string
	)
	if index < len(dial.Addrs) {
		addr = dial.Addrs[index]
	} else {
		addr = dial.Addrs[0]
	}

	if !dial.TLS.Enable {
		conn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		return conn, err
	} else {
		// load all certs to dial
		certs := []tls.Certificate{}
		for _, certFile := range dial.TLS.Certs {
			cert, err := tls.LoadX509KeyPair(certFile.Cert, certFile.Key)
			if err != nil {
				log.Errorf("dial, tls load x509 cert err: %s, cert: %s, key: %s", err, certFile.Cert, certFile.Key)
				continue
			}
			certs = append(certs, cert)
		}

		if !dial.TLS.MTLS {
			// tls
			conn, err := tls.Dial(network, addr, &tls.Config{
				Certificates: certs,
				// it's user's call to verify the server certs or not.
				InsecureSkipVerify: dial.TLS.InsecureSkipVerify,
			})
			if err != nil {
				log.Errorf("tls dial err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
			return conn, nil
		} else {
			// mtls, dial with our certs
			// load all ca certs to pool
			caPool := x509.NewCertPool()
			for _, caFile := range dial.TLS.CACerts {
				ca, err := os.ReadFile(caFile)
				if err != nil {
					log.Errorf("dial read ca cert err: %s, file: %s", err, caFile)
					return nil, err
				}
				if !caPool.AppendCertsFromPEM(ca) {
					log.Warnf("dial append ca cert to ca pool err: %s, file: %s", err, caFile)
					continue
				}
			}
			conn, err := tls.Dial(network, addr, &tls.Config{
				Certificates: certs,
				// we should not skip the verify.
				InsecureSkipVerify: dial.TLS.InsecureSkipVerify,
				RootCAs:            caPool,
			})
			if err != nil {
				log.Errorf("dial tls dial err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
			return conn, nil
		}
	}
}
