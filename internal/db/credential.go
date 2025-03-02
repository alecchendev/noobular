package db

import (
	"database/sql"
	"errors"
)

const createCredentialTable = `
create table if not exists credentials (
	id blob primary key,
	user_id integer not null,
	public_key blob not null,
	attestation_type text not null,
	transport blob not null, --json
	flags blob not null, --json
	authenticator blob not null, --json
	foreign key (user_id) references users(id) on delete cascade
);
`

type Credential struct {
	Id              []byte
	UserId          int64
	PublicKey       []byte
	AttestationType string
	Transport       []byte
	Flags           []byte
	Authenticator   []byte
}

func (c *DbClient) InsertCredential(credential Credential) error {
	const insertCredentialQuery = `
		insert into credentials(id, user_id, public_key, attestation_type, transport, flags, authenticator)
		values(?, ?, ?, ?, ?, ?, ?);
		`
	_, err := c.db.Exec(insertCredentialQuery, credential.Id, credential.UserId, credential.PublicKey, credential.AttestationType, credential.Transport, credential.Flags, credential.Authenticator)
	return err
}

func (c *DbClient) UpdateCredential(credential Credential) error {
	const updateCredentialQuery = `
		update credentials
		set public_key = ?, attestation_type = ?, transport = ?, flags = ?, authenticator = ?
		where id = ?;
		`
	_, err := c.db.Exec(updateCredentialQuery, credential.PublicKey, credential.AttestationType, credential.Transport, credential.Flags, credential.Authenticator, credential.Id)
	return err
}

func (c *DbClient) GetCredentialByUserId(userId int64) (Credential, bool, error) {
	const getCredentialsByUserIdQuery = `
		select id, user_id, public_key, attestation_type, transport, flags, authenticator from credentials where user_id = ?;
		`
	var credential Credential
	res := c.db.QueryRow(getCredentialsByUserIdQuery, userId)
	err := res.Scan(&credential.Id, &credential.UserId, &credential.PublicKey, &credential.AttestationType, &credential.Transport, &credential.Flags, &credential.Authenticator)
	if errors.Is(err, sql.ErrNoRows) {
		return Credential{}, false, nil
	}
	if err != nil {
		return Credential{}, false, err
	}
	return credential, true, nil
}
