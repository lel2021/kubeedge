/*
Copyright 2020 The KubeEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpserver

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"net"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	hubconfig "github.com/kubeedge/kubeedge/cloud/pkg/cloudhub/config"
	"github.com/kubeedge/kubeedge/common/constants"
)

// SignCerts creates server's certificate and key
func SignCerts() ([]byte, []byte, error) {
	cfg := &certutil.Config{
		CommonName:   constants.ProjectName,
		Organization: []string{constants.ProjectName},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: certutil.AltNames{
			DNSNames: hubconfig.Config.DNSNames,
			IPs:      getIps(hubconfig.Config.AdvertiseAddress),
		},
	}

	certDER, keyDER, err := NewCloudCoreCertDERandKey(cfg)
	if err != nil {
		return nil, nil, err
	}

	return certDER, keyDER, nil
}

func getIps(advertiseAddress []string) (Ips []net.IP) {
	for _, addr := range advertiseAddress {
		Ips = append(Ips, net.ParseIP(addr))
	}
	return
}

// GenerateToken will create a token consisting of caHash and jwt Token and save it to secret
func GenerateToken() error {
	t := time.NewTicker(time.Hour * hubconfig.Config.CloudHub.TokenRefreshDuration)
	go func() {
		for {
			refreshedCaHashToken := refreshToken()
			if err := CreateTokenSecret([]byte(refreshedCaHashToken)); err != nil {
				klog.Exitf("Failed to create the ca token for edgecore register, err: %v", err)
			}
			klog.Info("Succeed to creating token")
			<-t.C
		}
	}()
	return nil
}

func refreshToken() string {
	// set double TokenRefreshDuration as expirationTime, which can guarantee that the validity period
	// of the token obtained at anytime is greater than or equal to TokenRefreshDuration
	claims := &jwt.StandardClaims{}
	expirationTime := time.Now().Add(time.Hour * hubconfig.Config.CloudHub.TokenRefreshDuration * 2)
	claims.ExpiresAt = expirationTime.Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	keyPEM := getCaKey()
	tokenString, err := token.SignedString(keyPEM)
	if err != nil {
		klog.Errorf("Failed to generate token signed by caKey, err: %v", err)
	}
	caHash := getCaHash()
	//put caHash in token
	caHashAndToken := strings.Join([]string{caHash, tokenString}, ".")
	return caHashAndToken
}

// getCaHash gets ca-hash
func getCaHash() string {
	caDER := hubconfig.Config.Ca
	digest := sha256.Sum256(caDER)
	return hex.EncodeToString(digest[:])
}

// getCaKey gets caKey to encrypt token
func getCaKey() []byte {
	caKey := hubconfig.Config.CaKey
	return caKey
}
