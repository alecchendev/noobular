package db

import (
	"database/sql"
	"errors"
	"log"
)

const createUserTable = `
create table if not exists users (
	id integer primary key,
	username text not null unique
);
`

type User struct {
	Id       int64
	Username string
}

func NewUser(id int64, username string) User {
	return User{id, username}
}

func (c DbClient) CreateUser(username string) (User, error) {
	res, err := c.tx.Exec("insert into users(username) values(?);", username)
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return NewUser(id, username), nil
}

func (c DbClient) GetUser(userId int64) (User, bool, error) {
	row := c.tx.QueryRow("select id, username from users where id = ?;", userId)
	var username string
	err := row.Scan(&userId, &username)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return NewUser(userId, username), true, nil
}

func (c DbClient) GetUserByUsername(username string) (User, bool, error) {
	row := c.tx.QueryRow("select id, username from users where username = ?;", username)
	var id int64
	err := row.Scan(&id, &username)
	if errors.Is(err, sql.ErrNoRows) {
		log.Println("No user with username:", username)
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return NewUser(id, username), true, nil
}
