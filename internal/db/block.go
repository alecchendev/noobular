package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Blocks are pieces of a module, either a question or piece of content.
const createBlockTable = `
create table if not exists blocks (
	id integer primary key autoincrement,
	module_id integer not null,
	block_index integer not null,
	block_type text not null,
	foreign key (module_id) references modules(id) on delete cascade
);
`

type BlockType string

const (
	QuestionBlockType BlockType = "question"
	ContentBlockType  BlockType = "content"
)

type Block struct {
	Id         int
	ModuleId   int
	BlockIndex int
	BlockType  BlockType
}

const insertBlockQuery = `
insert into blocks(module_id, block_index, block_type)
values(?, ?, ?);
`

func InsertBlock(tx *sql.Tx, moduleId int, blockIdx int, blockType BlockType) (int64, error) {
	res, err := tx.Exec(insertBlockQuery, moduleId, blockIdx, blockType)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const getBlocksQuery = `
select b.id, b.module_id, b.block_index, b.block_type
from blocks b
where b.module_id = ?
order by b.block_index;
`

func (c *DbClient) GetBlocks(moduleId int) ([]Block, error) {
	blockRows, err := c.db.Query(getBlocksQuery, moduleId)
	if err != nil {
		return nil, err
	}
	defer blockRows.Close()
	blocks := []Block{}
	for blockRows.Next() {
		var block Block
		err := blockRows.Scan(&block.Id, &block.ModuleId, &block.BlockIndex, &block.BlockType)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if err := blockRows.Err(); err != nil {
		return nil, err
	}
	return blocks, nil
}

const getBlockQuery = `
select b.id, b.module_id, b.block_index, b.block_type
from blocks b
where b.block_index = ?
and b.module_id = ?;
`

func (c *DbClient) GetBlock(moduleId int, blockIdx int) (Block, error) {
	blockRow := c.db.QueryRow(getBlockQuery, blockIdx, moduleId)
	block := Block{}
	err := blockRow.Scan(&block.Id, &block.ModuleId, &block.BlockIndex, &block.BlockType)
	if err != nil {
		return Block{}, err
	}
	return block, nil
}

const getBlockCountQuery = `
select count(*)
from blocks b
where b.module_id = ?;
`

func (c *DbClient) GetBlockCount(moduleId int) (int, error) {
	row := c.db.QueryRow(getBlockCountQuery, moduleId)
	var blockCount int
	err := row.Scan(&blockCount)
	if err != nil {
		return 0, err
	}
	return blockCount, nil
}

