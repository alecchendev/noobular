package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Blocks are pieces of a module, either a question or piece of content.
const createBlockTable = `
create table if not exists blocks (
	id integer primary key autoincrement,
	module_version_id integer not null,
	block_index integer not null,
	block_type text not null,
	foreign key (module_version_id) references module_versions(id) on delete cascade,
	constraint block_ unique(module_version_id, block_index) on conflict fail
);
`

// TODO: turn this into an enum
type BlockType string

const (
	ContentBlockType  BlockType = "content"
	KnowledgePointBlockType BlockType = "knowledge_point"
)

type Block struct {
	Id              int
	ModuleVersionId int64
	BlockIndex      int
	BlockType       BlockType
}

func NewBlock(id int, moduleVersionId int64, blockIdx int, blockType BlockType) Block {
	return Block{id, moduleVersionId, blockIdx, blockType}
}

const insertBlockQuery = `
insert into blocks(module_version_id, block_index, block_type)
values(?, ?, ?);
`

func InsertBlock(tx *sql.Tx, moduleVersionId int64, blockIdx int, blockType BlockType) (int64, error) {
	res, err := tx.Exec(insertBlockQuery, moduleVersionId, blockIdx, blockType)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const getBlocksQuery = `
select b.id, b.module_version_id, b.block_index, b.block_type
from blocks b
where b.module_version_id = ?
order by b.block_index;
`

func (c *DbClient) GetBlocks(moduleVersionId int64) ([]Block, error) {
	blockRows, err := c.db.Query(getBlocksQuery, moduleVersionId)
	if err != nil {
		return nil, err
	}
	defer blockRows.Close()
	blocks := []Block{}
	for blockRows.Next() {
		var block Block
		err := blockRows.Scan(&block.Id, &block.ModuleVersionId, &block.BlockIndex, &block.BlockType)
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
select b.id, b.module_version_id, b.block_index, b.block_type
from blocks b
where b.block_index = ?
and b.module_version_id = ?;
`

func (c *DbClient) GetBlock(moduleVersionId int64, blockIdx int) (Block, error) {
	blockRow := c.db.QueryRow(getBlockQuery, blockIdx, moduleVersionId)
	block := Block{}
	err := blockRow.Scan(&block.Id, &block.ModuleVersionId, &block.BlockIndex, &block.BlockType)
	if err != nil {
		return Block{}, err
	}
	return block, nil
}

const getBlockCountQuery = `
select count(*)
from blocks b
where b.module_version_id = ?;
`

func (c *DbClient) GetBlockCount(moduleVersionId int64) (int, error) {
	row := c.db.QueryRow(getBlockCountQuery, moduleVersionId)
	var blockCount int
	err := row.Scan(&blockCount)
	if err != nil {
		return 0, err
	}
	return blockCount, nil
}

const deleteBlockQuery = `
delete from blocks
where module_version_id = ?;
`

func DeleteBlocks(tx *sql.Tx, moduleVersionId int64) error {
	_, err := tx.Exec(deleteBlockQuery, moduleVersionId)
	return err
}
