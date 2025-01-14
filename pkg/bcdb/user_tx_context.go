// Copyright IBM Corp. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package bcdb

import (
	"github.com/golang/protobuf/proto"
	"github.com/IBM-Blockchain/bcdb-server/pkg/constants"
	"github.com/IBM-Blockchain/bcdb-server/pkg/cryptoservice"
	"github.com/IBM-Blockchain/bcdb-server/pkg/types"
)

// UsersTxContext transaction context to operate with
// user management related transactions:
// 1. Add user's record
// 2. Get user's record
// 3. Delete user's record
// 4. Alternate user's ACLs
type UsersTxContext interface {
	// Embed general abstraction
	TxContext
	// PutUser introduce new user into database
	PutUser(user *types.User, acl *types.AccessControl) error
	// GetUser obtain user's record from database
	GetUser(userID string) (*types.User, error)
	// RemoveUser delete existing user from the database
	RemoveUser(userID string) error
}

type userTxContext struct {
	*commonTxContext
	userReads   []*types.UserRead
	userWrites  []*types.UserWrite
	userDeletes []*types.UserDelete
}

func (u *userTxContext) Commit(sync bool) (string, *types.TxReceipt, error) {
	return u.commit(u, constants.PostUserTx, sync)
}

func (u *userTxContext) Abort() error {
	return u.abort(u)
}

func (u *userTxContext) PutUser(user *types.User, acl *types.AccessControl) error {
	if u.txSpent {
		return ErrTxSpent
	}

	// TODO: decide whenever we going to support read your own writes
	u.userWrites = append(u.userWrites, &types.UserWrite{
		User: user,
		ACL:  acl,
	})
	return nil
}

func (u *userTxContext) GetUser(userID string) (*types.User, error) {
	if u.txSpent {
		return nil, ErrTxSpent
	}

	path := constants.URLForGetUser(userID)
	res := &types.GetUserResponse{}
	err := u.handleRequest(path, &types.GetUserQuery{
		UserID:       u.userID,
		TargetUserID: userID,
	}, res)
	if err != nil {
		u.logger.Errorf("failed to execute user query, path = %s, due to %s", path, err)
		return nil, err
	}
	u.userReads = append(u.userReads, &types.UserRead{
		UserID:  userID,
		Version: res.GetMetadata().GetVersion(),
	})

	return res.GetUser(), nil
}

func (u *userTxContext) RemoveUser(userID string) error {
	if u.txSpent {
		return ErrTxSpent
	}

	u.userDeletes = append(u.userDeletes, &types.UserDelete{
		UserID: userID,
	})
	return nil
}

func (u *userTxContext) composeEnvelope(txID string) (proto.Message, error) {
	payload := &types.UserAdministrationTx{
		UserID:      u.userID,
		TxID:        txID,
		UserReads:   u.userReads,
		UserWrites:  u.userWrites,
		UserDeletes: u.userDeletes,
	}

	signature, err := cryptoservice.SignTx(u.signer, payload)
	if err != nil {
		return nil, err
	}

	return &types.UserAdministrationTxEnvelope{
		Payload:   payload,
		Signature: signature,
	}, nil
}

func (u *userTxContext) cleanCtx() {
	u.userDeletes = []*types.UserDelete{}
	u.userWrites = []*types.UserWrite{}
	u.userReads = []*types.UserRead{}
}
