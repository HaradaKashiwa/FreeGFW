package services

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"net"
	"os"
	"time"

	"freegfw/database"
	"freegfw/models"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u *MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

func ApplyCertificate(domain, email string) error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	myUser := MyUser{
		Email: email,
		key:   privateKey,
	}

	config := lego.NewConfig(&myUser)

	// This CA URL is configured for a local dev instance of Boulder running in Docker in a VM.
	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(config)
	if err != nil {
		return err
	}

	// Use HTTP-01 challenge provider server listening on port 80
	err = client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))
	if err != nil {
		return err
	}

	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return err
	}
	myUser.Registration = reg

	var certificates *certificate.Resource

	ipAddr := net.ParseIP(domain)
	if ipAddr != nil {
		// For IPs, we must NOT put the IP in CommonName.
		// We generate a CSR manually with the IP in SANs (IPAddresses).
		// Note: The certificate key MUST be different from the account key.
		certPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}

		csrTemplate := &x509.CertificateRequest{
			IPAddresses: []net.IP{ipAddr},
		}
		csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, certPrivateKey)
		if err != nil {
			return err
		}

		csr, err := x509.ParseCertificateRequest(csrBytes)
		if err != nil {
			return err
		}

		request := certificate.ObtainForCSRRequest{
			CSR:     csr,
			Bundle:  true,
			Profile: "shortlived",
		}
		certificates, err = client.Certificate.ObtainForCSR(request)
		if err != nil {
			return err
		}

		encodedKey, err := x509.MarshalECPrivateKey(certPrivateKey)
		if err != nil {
			return err
		}
		certificates.PrivateKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encodedKey})
	} else {
		request := certificate.ObtainRequest{
			Domains: []string{domain},
			Bundle:  true,
			Profile: "shortlived",
		}
		certificates, err = client.Certificate.Obtain(request)
		if err != nil {
			return err
		}
	}

	// ensure data dir exists
	os.MkdirAll("data", 0755)

	err = os.WriteFile("data/certificate.crt", certificates.Certificate, 0644)
	if err != nil {
		return err
	}

	err = os.WriteFile("data/private.key", certificates.PrivateKey, 0600)
	if err != nil {
		return err
	}

	return nil
}

func StartCertificateRenewalLoop() {
	go func() {
		// Initial check after startup delay to ensure DB is ready
		time.Sleep(1 * time.Minute)
		CheckAndRenewCertificate()
	}()

	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		CheckAndRenewCertificate()
	}
}

func CheckAndRenewCertificate() {
	certFile := "data/certificate.crt"
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Println("Failed to parse certificate:", err)
		return
	}

	// Check if certificate expires in less than 24 hours
	if time.Until(cert.NotAfter) < 24*time.Hour {
		log.Println("Certificate expires in less than 24 hours. Attempting renewal...")

		var emailSetting models.Setting
		database.DB.Where("key = ?", "letsencrypt_email").Limit(1).Find(&emailSetting)

		var domainSetting models.Setting
		database.DB.Where("key = ?", "letsencrypt_domain").Limit(1).Find(&domainSetting)

		var email string
		if len(emailSetting.Value) > 0 {
			json.Unmarshal(emailSetting.Value, &email)
		}

		var domain string
		if len(domainSetting.Value) > 0 {
			json.Unmarshal(domainSetting.Value, &domain)
		}

		if email != "" && domain != "" {
			if err := ApplyCertificate(domain, email); err != nil {
				log.Println("Failed to renew certificate:", err)
			} else {
				log.Println("Certificate renewed successfully. Requesting server restart.")
				go func() {
					time.Sleep(5 * time.Second) // Delay to ensure other operations complete
					RestartChan <- struct{}{}
				}()
				// Update timestamp
				t := time.Now().UnixMilli()
				tBytes, _ := json.Marshal(t)

				var s models.Setting
				if database.DB.Where("key = ?", "letsencrypt_updated_at").Limit(1).Find(&s).RowsAffected == 0 {
					s = models.Setting{Key: "letsencrypt_updated_at"}
				}
				s.Value = models.JSON(tBytes)
				database.DB.Save(&s)
			}
		} else {
			log.Println("Cannot renew certificate: Email or Domain not found in settings.")
		}
	}
}
