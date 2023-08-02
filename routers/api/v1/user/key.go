// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	std_ctx "context"
	"net/http"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/convert"
)

// appendPrivateInformation appends the owner and key type information to api.PublicKey
func appendPrivateInformation(ctx std_ctx.Context, apiKey *api.PublicKey, key *asymkey_model.PublicKey, defaultUser *user_model.User) (*api.PublicKey, error) {
	if key.Type == asymkey_model.KeyTypeDeploy {
		apiKey.KeyType = "deploy"
	} else if key.Type == asymkey_model.KeyTypeUser {
		apiKey.KeyType = "user"

		if defaultUser.ID == key.OwnerID {
			apiKey.Owner = convert.ToUser(ctx, defaultUser, defaultUser)
		} else {
			user, err := user_model.GetUserByID(ctx, key.OwnerID)
			if err != nil {
				return apiKey, err
			}
			apiKey.Owner = convert.ToUser(ctx, user, user)
		}
	} else {
		apiKey.KeyType = "unknown"
	}
	apiKey.ReadOnly = key.Mode == perm.AccessModeRead
	return apiKey, nil
}

func composePublicKeysAPILink() string {
	return setting.AppURL + "api/v1/user/keys/"
}

func listPublicKeys(ctx *context.APIContext, user *user_model.User) {
	var keys []*asymkey_model.PublicKey
	var err error
	var count int

	fingerprint := ctx.FormString("fingerprint")
	username := ctx.Params("username")

	if fingerprint != "" {
		// Querying not just listing
		if username != "" {
			// Restrict to provided uid
			keys, err = asymkey_model.SearchPublicKey(user.ID, fingerprint)
		} else {
			// Unrestricted
			keys, err = asymkey_model.SearchPublicKey(0, fingerprint)
		}
		count = len(keys)
	} else {
		total, err2 := asymkey_model.CountPublicKeys(user.ID)
		if err2 != nil {
			ctx.InternalServerError(err)
			return
		}
		count = int(total)

		// Use ListPublicKeys
		keys, err = asymkey_model.ListPublicKeys(user.ID, utils.GetListOptions(ctx))
	}

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListPublicKeys", err)
		return
	}

	apiLink := composePublicKeysAPILink()
	apiKeys := make([]*api.PublicKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToPublicKey(apiLink, keys[i])
		if ctx.Doer.IsAdmin || ctx.Doer.ID == keys[i].OwnerID {
			apiKeys[i], _ = appendPrivateInformation(ctx, apiKeys[i], keys[i], user)
		}
	}

	ctx.SetTotalCountHeader(int64(count))
	ctx.JSON(http.StatusOK, &apiKeys)
}

// ListMyPublicKeys list all of the authenticated user's public keys
func ListMyPublicKeys(ctx *context.APIContext) {
	// swagger:operation GET /user/keys user userCurrentListKeys
	// ---
	// summary: List the authenticated user's public keys
	// parameters:
	// - name: fingerprint
	//   in: query
	//   description: fingerprint of the key
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/PublicKeyList"

	listPublicKeys(ctx, ctx.Doer)
}

// ListPublicKeys list the given user's public keys
func ListPublicKeys(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/keys user userListKeys
	// ---
	// summary: List the given user's public keys
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: fingerprint
	//   in: query
	//   description: fingerprint of the key
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PublicKeyList"

	listPublicKeys(ctx, ctx.ContextUser)
}

// GetPublicKey get a public key
func GetPublicKey(ctx *context.APIContext) {
	// swagger:operation GET /user/keys/{id} user userCurrentGetKey
	// ---
	// summary: Get a public key
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PublicKey"
	//   "404":
	//     "$ref": "#/responses/notFound"

	key, err := asymkey_model.GetPublicKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPublicKeyByID", err)
		}
		return
	}

	apiLink := composePublicKeysAPILink()
	apiKey := convert.ToPublicKey(apiLink, key)
	if ctx.Doer.IsAdmin || ctx.Doer.ID == key.OwnerID {
		apiKey, _ = appendPrivateInformation(ctx, apiKey, key, ctx.Doer)
	}
	ctx.JSON(http.StatusOK, apiKey)
}

// CreateUserPublicKey creates new public key to given user by ID.
func CreateUserPublicKey(ctx *context.APIContext, form api.CreateKeyOption, uid int64) {
	content, err := asymkey_model.CheckPublicKeyString(form.Key)
	if err != nil {
		repo.HandleCheckKeyStringError(ctx, err)
		return
	}

	key, err := asymkey_model.AddPublicKey(uid, form.Title, content, 0)
	if err != nil {
		repo.HandleAddKeyError(ctx, err)
		return
	}
	apiLink := composePublicKeysAPILink()
	apiKey := convert.ToPublicKey(apiLink, key)
	if ctx.Doer.IsAdmin || ctx.Doer.ID == key.OwnerID {
		apiKey, _ = appendPrivateInformation(ctx, apiKey, key, ctx.Doer)
	}
	ctx.JSON(http.StatusCreated, apiKey)
}

// CreatePublicKey create one public key for me
func CreatePublicKey(ctx *context.APIContext) {
	// swagger:operation POST /user/keys user userCurrentPostKey
	// ---
	// summary: Create a public key
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateKeyOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PublicKey"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateKeyOption)
	CreateUserPublicKey(ctx, *form, ctx.Doer.ID)
}

// DeletePublicKey delete one public key
func DeletePublicKey(ctx *context.APIContext) {
	// swagger:operation DELETE /user/keys/{id} user userCurrentDeleteKey
	// ---
	// summary: Delete a public key
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	id := ctx.ParamsInt64(":id")
	externallyManaged, err := asymkey_model.PublicKeyIsExternallyManaged(id)
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "PublicKeyIsExternallyManaged", err)
		}
		return
	}

	if externallyManaged {
		ctx.Error(http.StatusForbidden, "", "SSH Key is externally managed for this user")
		return
	}

	if err := asymkey_service.DeletePublicKey(ctx.Doer, id); err != nil {
		if asymkey_model.IsErrKeyAccessDenied(err) {
			ctx.Error(http.StatusForbidden, "", "You do not have access to this key")
		} else {
			ctx.Error(http.StatusInternalServerError, "DeletePublicKey", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
