package server

import (
	"encoding/binary"
	"encoding/json"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"noobular/internal/db"
)

type WebAuthnUser struct {
	User        db.User
	Credentials []webauthn.Credential
}

func NewWebAuthnUser(user db.User) WebAuthnUser {
	return WebAuthnUser{user, []webauthn.Credential{}}
}

func NewWebAuthnUserWithCred(user db.User, credential webauthn.Credential) WebAuthnUser {
	return WebAuthnUser{user, []webauthn.Credential{credential}}
}

// Implement webauthn.User interface

func (u *WebAuthnUser) WebAuthnID() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(u.User.Id))
	return b
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Username
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.User.Username
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

func NewCredential(userId int64, credential webauthn.Credential) (db.Credential, error) {
	if credential.Transport == nil {
		credential.Transport = []protocol.AuthenticatorTransport{}
	}
	transport, err := json.Marshal(credential.Transport)
	if err != nil {
		return db.Credential{}, err
	}
	flags, err := json.Marshal(credential.Flags)
	if err != nil {
		return db.Credential{}, err
	}
	authenticator, err := json.Marshal(credential.Authenticator)
	if err != nil {
		return db.Credential{}, err
	}
	return db.Credential{
		Id:              credential.ID,
		UserId:          userId,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Transport:       transport,
		Flags:           flags,
		Authenticator:   authenticator,
	}, nil
}

func NewWebAuthnCredential(credential *db.Credential) (webauthn.Credential, error) {
	var transport []protocol.AuthenticatorTransport
	if err := json.Unmarshal(credential.Transport, &transport); err != nil {
		return webauthn.Credential{}, err
	}
	var flags webauthn.CredentialFlags
	if err := json.Unmarshal(credential.Flags, &flags); err != nil {
		return webauthn.Credential{}, err
	}
	var authenticator webauthn.Authenticator
	if err := json.Unmarshal(credential.Authenticator, &authenticator); err != nil {
		return webauthn.Credential{}, err
	}
	return webauthn.Credential{
		ID:              credential.Id,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Transport:       transport,
		Flags:           flags,
		Authenticator:   authenticator,
	}, nil
}
