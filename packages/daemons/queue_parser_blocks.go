/*---------------------------------------------------------------------------------------------
 *  Copyright (c) IBAX. All rights reserved.
 *  See LICENSE in the project root for license information.
 *--------------------------------------------------------------------------------------------*/

package daemons

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/IBAX-io/go-ibax/packages/conf"
	"github.com/IBAX-io/go-ibax/packages/conf/syspar"
	"github.com/IBAX-io/go-ibax/packages/consts"
 *  - insert the frontal data from a new chain
 *  - if there is no error, then roll back our data from the blocks
 *  - and insert new data
 *  - if there are errors, then roll back to the former data
 * */

// QueueParserBlocks parses and applies blocks from the queue
func QueueParserBlocks(ctx context.Context, d *daemon) error {
	if atomic.CompareAndSwapUint32(&d.atomic, 0, 1) {
		defer atomic.StoreUint32(&d.atomic, 0)
	} else {
		return nil
	}
	DBLock()
	defer DBUnlock()

	infoBlock := &model.InfoBlock{}
	_, err := infoBlock.Get()
	if err != nil {
		d.logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("getting info block")
		return err
	}
	queueBlock := &model.QueueBlock{}
	_, err = queueBlock.Get()
	if err != nil {
		d.logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("getting queue block")
		return err
	}
	if len(queueBlock.Hash) == 0 {
		d.logger.WithFields(log.Fields{"type": consts.NotFound}).Debug("queue block not found")
		return err
	}

	// check if the block gets in the rollback_blocks_1 limit
	if queueBlock.BlockID > infoBlock.BlockID+syspar.GetRbBlocks1() {
		queueBlock.DeleteOldBlocks()
		return utils.ErrInfo("rollback_blocks_1")
	}

	// is it old block in queue ?
	if queueBlock.BlockID <= infoBlock.BlockID {
		queueBlock.DeleteOldBlocks()
		return utils.ErrInfo(fmt.Errorf("old block %d <= %d", queueBlock.BlockID, infoBlock.BlockID))
	}

	if queueBlock.HonorNodeID == conf.Config.KeyID {
		d.logger.WithFields(log.Fields{"type": consts.DuplicateObject}).Debug("queueBlock generated by myself", queueBlock.BlockID)
		return utils.ErrInfo(fmt.Errorf("queueBlock generated by myself: %d", queueBlock.BlockID))
	}

	nodeHost, err := syspar.GetNodeHostByPosition(queueBlock.HonorNodeID)
	if err != nil {
		queueBlock.DeleteQueueBlockByHash()
		return utils.ErrInfo(err)
	}
	blockID := queueBlock.BlockID

	host := utils.GetHostPort(nodeHost)
	// update our chain till maxBlockID from the host
	return UpdateChain(ctx, d, host, blockID)
}
