package exporter

import (
	"encoding/hex"
	"errors"

	"github.com/wenzhenxi/gorsa"
)

// AompPasswordDecrypt, decrypt the password passed from aomp
func AompPasswordDecrypt(engypted, aompPubKey, appPrivKey string) (string, error) {
	privOut, err := PubKeyDECRYPT(engypted, aompPubKey)
	if err != nil {
		return "", err
	}

	privOut, err = PriKeyDECRYPT(privOut, appPrivKey)
	if err != nil {
		return "", err
	}
	return string(privOut), nil
}

// decrypt
func PubKeyDECRYPT(engypted, pubKey string) ([]byte, error) {
	out, err := hex.DecodeString(engypted)
	if err != nil {
		return nil, errors.New("hex decode error:" + err.Error())
	}
	grsa := gorsa.RSASecurity{}
	grsa.SetPublicKey(pubKey)

	rsadata, err := grsa.PubKeyDECRYPT(out)
	if err != nil {
		return nil, errors.New("public key decrypt error:" + err.Error())
	}

	return rsadata, nil
}

func PriKeyDECRYPT(rsadata []byte, privKey string) ([]byte, error) {
	grsa := gorsa.RSASecurity{}
	grsa.SetPrivateKey(privKey)

	rsadata2, err := grsa.PriKeyDECRYPT(rsadata)
	if err != nil {
		return nil, errors.New("private key decrypt error:" + err.Error())
	}

	return rsadata2, nil
}
