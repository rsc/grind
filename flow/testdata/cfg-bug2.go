package p

import "encoding/pem"

func f() {
	var cert Certificate
	var certDERBlock *pem.Block
	fail := func(err error) (Certificate, error) { return Certificate{}, err }
	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		}
		break
	}
}
