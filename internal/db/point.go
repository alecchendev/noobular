package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const createPointTable = `
create table if not exists points (
	id integer primary key autoincrement,
	user_id integer not null,
	module_id integer not null,
	count integer not null,
	created_at datetime not null,
	foreign key (user_id) references users(id) on delete cascade,
	foreign key (module_id) references modules(id) on delete cascade,
	constraint point_ unique(user_id, module_id) on conflict fail
);
`

type Point struct {
	Id      int
	UserId  int64
	ModuleId int
	Count int
	CreatedAt time.Time
}

func NewPoint(id int, userId int64, moduleId int, count int, createdAt time.Time) Point {
	return Point{id, userId, moduleId, count, createdAt}
}

const insertPointQuery = `
insert into points(user_id, module_id, count, created_at)
values(?, ?, ?, ?);
`

func InsertPoint(tx *sql.Tx, userId int64, moduleId int, count int) (Point, error) {
	now := time.Now().UTC()
	res, err := tx.Exec(insertPointQuery, userId, moduleId, count, now)
	if err != nil {
		return Point{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Point{}, err
	}
	return NewPoint(int(id), userId, moduleId, count, now), nil
}

const getPoint = `
select p.id, p.user_id, p.module_id, p.count, p.created_at
from points p
where p.user_id = ? and p.module_id = ?;
`

func (c *DbClient) GetPoint(userId int64, moduleId int) (Point, error) {
	row := c.db.QueryRow(getPoint, userId, moduleId)
	var point Point
	var createdAt string
	err := row.Scan(&point.Id, &point.UserId, &point.ModuleId, &point.Count, &createdAt)
	if err != nil {
		return Point{}, err
	}
	point.CreatedAt, err = time.Parse("2006-01-02T15:04:05Z", createdAt)
	if err != nil {
		return Point{}, err
	}
	return point, nil
}
