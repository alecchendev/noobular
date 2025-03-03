package db

const createSessionTable = `
create table if not exists sessions (
	id integer primary key autoincrement,
	user_id integer not null,
	session_data blob not null,
	foreign key (user_id) references users(id) on delete cascade
);
`

func (c *DbClient) InsertSession(userId int64, sessionData []byte) error {
	const deleteSessionQuery = `
		delete from sessions where user_id = ?;
		`
	_, err := c.tx.Exec(deleteSessionQuery, userId)
	if err != nil {
		return err
	}
	const insertSessionQuery = `
		insert into sessions(user_id, session_data)
		values(?, ?);
		`
	_, err = c.tx.Exec(insertSessionQuery, userId, sessionData)
	if err != nil {
		return err
	}
	return nil
}

func (c *DbClient) GetSession(userId int64) ([]byte, error) {
	const getSessionQuery = `
		select session_data from sessions where user_id = ?;
		`
	var sessionData []byte
	res := c.tx.QueryRow(getSessionQuery, userId)
	err := res.Scan(&sessionData)
	if err != nil {
		return nil, err
	}
	return sessionData, nil
}
