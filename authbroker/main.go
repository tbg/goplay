package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/nacl/sign"
)

type AuthBroker struct {
	publicKey  *[32]byte
	privateKey *[64]byte
}

func (ab *AuthBroker) MakeToken(tenantID uint64, expiration time.Time) []byte {
	var message []byte
	message = append(message, proto.EncodeVarint(tenantID)...)
	message = append(message, proto.EncodeVarint(uint64(expiration.UnixNano()))...)
	return sign.Sign(nil, message, ab.privateKey)
}

const extension = 10 * time.Second

func (ab *AuthBroker) RefreshToken(tok []byte, now time.Time) ([]byte, error) {
	tenantID, expiration, err := verifyToken(tok, now, ab.publicKey)
	if err != nil {
		return nil, err
	}
	return ab.MakeToken(tenantID, expiration.Add(extension)), nil
}

// This would also be called by the KV layer.
func verifyToken(tok []byte, now time.Time, pubKey *[32]byte) (tenantID uint64, expiration time.Time, _ error) {
	now = now.UTC()
	b, ok := sign.Open(nil, tok, pubKey)
	if !ok {
		return 0, time.Time{}, errors.New("invalid token")
	}
	tenantID, n := proto.DecodeVarint(b)
	if n == 0 {
		return 0, time.Time{}, errors.New("unable to decode tenantID")
	}
	b = b[n:]
	nanos, n := proto.DecodeVarint(b)
	if n != len(b) {
		return 0, time.Time{}, errors.New("unable to decode expiration")
	}

	expiration = time.Unix(0, int64(nanos)).UTC()

	if expiration.Before(now) {
		return 0, time.Time{}, errors.New("token is expired")
	}

	return tenantID, expiration, nil
}

func ts(sec int64) time.Time {
	return time.Unix(sec, 0)
}

func main() {
	publicKey, privateKey, err := sign.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	ab := &AuthBroker{publicKey: publicKey, privateKey: privateKey}

	tok := ab.MakeToken(129, ts(100))
	fmt.Printf("[%d] %q\n", len(tok), tok)
	tenantID, expiration, err := verifyToken(tok, ts(99), publicKey)
	if err != nil {
		panic(err)
	}
	if tenantID != 129 {
		panic(tenantID)
	}
	if !expiration.Equal(ts(100)) {
		panic(expiration)
	}

	_, _, err = verifyToken(tok, ts(101), publicKey)
	if err == nil {
		panic("wanted error")
	}
	fmt.Println("verifying expired token: ", err)

	_, err = ab.RefreshToken(tok, ts(101))
	if err == nil {
		panic("wanted error")
	}
	fmt.Println("refreshing expired token: ", err)

	tok, err = ab.RefreshToken(tok, ts(98))
	if err != nil {
		panic(err)
	}

	tenantID, expiration, err = verifyToken(tok, ts(105), publicKey)
	if err != nil {
		panic(err)
	}
	if tenantID != 129 {
		panic(tenantID)
	}
	if !expiration.Equal(ts(100).Add(extension)) {
		panic(expiration)
	}

	fmt.Printf("[%d] %q\n", len(tok), tok)

	// Tamper proof.
	tok[3] = 12
	_, _, err = verifyToken(tok, ts(1), publicKey)
	if err == nil {
		panic("wanted error")
	}
}
