package db

import ()

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

const insertCredentialQuery = `
insert into credentials(id, user_id, public_key, attestation_type, transport, flags, authenticator)
values(?, ?, ?, ?, ?, ?, ?);
`

func (c *DbClient) InsertCredential(credential Credential) error {
	_, err := c.db.Exec(insertCredentialQuery, credential.Id, credential.UserId, credential.PublicKey, credential.AttestationType, credential.Transport, credential.Flags, credential.Authenticator)
	return err
}

const updateCredentialQuery = `
update credentials
set public_key = ?, attestation_type = ?, transport = ?, flags = ?, authenticator = ?
where id = ?;
`

func (c *DbClient) UpdateCredential(credential Credential) error {
	_, err := c.db.Exec(updateCredentialQuery, credential.PublicKey, credential.AttestationType, credential.Transport, credential.Flags, credential.Authenticator, credential.Id)
	return err
}

const getCredentialsByUserIdQuery = `
select id, user_id, public_key, attestation_type, transport, flags, authenticator from credentials where user_id = ?;
`

func (c *DbClient) GetCredentialByUserId(userId int64) (Credential, error) {
	var credential Credential
	res := c.db.QueryRow(getCredentialsByUserIdQuery, userId)
	err := res.Scan(&credential.Id, &credential.UserId, &credential.PublicKey, &credential.AttestationType, &credential.Transport, &credential.Flags, &credential.Authenticator)
	if err != nil {
		return Credential{}, err
	}
	return credential, nil
}

// Also just using a table for session storage

const createSessionTable = `
create table if not exists sessions (
	id integer primary key autoincrement,
	user_id integer not null,
	session_data blob not null,
	foreign key (user_id) references users(id) on delete cascade
);
`

const deleteSessionQuery = `
delete from sessions where user_id = ?;
`

const insertSessionQuery = `
insert into sessions(user_id, session_data)
values(?, ?);
`

func (c *DbClient) InsertSession(userId int64, sessionData []byte) error {
	_, err := c.db.Exec(deleteSessionQuery, userId)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(insertSessionQuery, userId, sessionData)
	if err != nil {
		return err
	}
	return nil
}

const getSessionQuery = `
select session_data from sessions where user_id = ?;
`

func (c *DbClient) GetSession(userId int64) ([]byte, error) {
	var sessionData []byte
	res := c.db.QueryRow(getSessionQuery, userId)
	err := res.Scan(&sessionData)
	if err != nil {
		return nil, err
	}
	return sessionData, nil
}
