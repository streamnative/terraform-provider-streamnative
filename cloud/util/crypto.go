package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwe"
	"github.com/pkg/errors"
)

func GenerateEncryptionKey() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func ExportPublicKey(key *rsa.PrivateKey) (*cloudv1alpha1.EncryptionKey, error) {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})
	return &cloudv1alpha1.EncryptionKey{
		PEM: string(pemKey),
	}, nil
}

func ExportPrivateKey(key *rsa.PrivateKey) string {
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(pemKey)
}

func ImportPrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func DecryptToken(priv *rsa.PrivateKey, encryptedToken cloudv1alpha1.EncryptedToken) (string, error) {
	if encryptedToken.JWE == nil {
		return "", errors.New("failed to decrypt the token (no JWE)")
	}
	token, err := jwe.Decrypt([]byte(*encryptedToken.JWE), jwe.WithKey(jwa.RSA_OAEP, priv))
	if err != nil {
		return "", errors.Wrapf(err, "failed to decrypt the token")
	}
	return string(token), nil
}
