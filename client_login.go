/*
Copyright (C) GMO GlobalSign, Inc. 2019 - All Rights Reserved.

Unauthorized copying of this file, via any medium is strictly prohibited.
No distribution/modification of whole or part thereof is allowed.

Proprietary and confidential.
*/

package hvclient

import (
	"context"
	"fmt"
	"time"
)

// loginResponse is the body of a successful response from the /login
// endpoint.
type loginResponse struct {
	AccessToken string `json:"access_token"`
}

const (
	// tokenLifetime is the assumed lifetime of an HVCA authentication token.
	// The HVCA API appears to not return any information confirming the
	// lifetime of the token, but at the time of writing the API documentation
	// states it to be 10 minutes. We here set it to nine minutes just to
	// leave some headroom.
	tokenLifetime = time.Minute * 9
)

// login logs into the HVCA server and stores the authentication token.
func (c *Client) login(ctx context.Context) error {
	var r loginResponse
	var _, err = c.makeRequest(ctx, c.loginRequest, &r)
	if err != nil {
		c.tokenReset()

		return fmt.Errorf("failed to login: %w", err)
	}

	c.tokenSet(r.AccessToken)

	return nil
}

// loginIfTokenHasExpired logs in if the stored authentication token has
// expired, or if there is no stored authentication token. To avoid
// unnecessary simultaneous re-logins, this method ensures only one goroutine
// at a time can perform a re-login operation via this method.
func (c *Client) loginIfTokenHasExpired(ctx context.Context) error {
	// Do nothing if the token is not yet believed to be expired.
	if !c.tokenHasExpired() {
		return nil
	}

	// Token is believed to be expired, so lock the login mutex to ensure only
	// one goroutine at a time can relogin. Note that it is perfectly safe for
	// one goroutine to call login (which doesn't acquire the login mutex) while
	// another calls this method (which does acquire it) - it's just somewhat
	// inefficient. Also note that access to the token is sychronized using
	// a different mutex, so attempting to acquire that mutex while holding
	// this one won't cause a deadlock.
	c.loginMtx.Lock()
	defer c.loginMtx.Unlock()

	// Check again if the token is believed to be expired, as another
	// goroutine may have acquired the login mutex before we did.
	if !c.tokenHasExpired() {
		return nil
	}

	return c.login(ctx)
}

// tokenHasExpired returns true if the stored authentication token is believed
// to be expired (or if there is no stored authentication token), indicating
// that another login is required.
func (c *Client) tokenHasExpired() bool {
	c.tokenMtx.RLock()
	defer c.tokenMtx.RUnlock()

	return time.Since(c.lastLogin) > tokenLifetime
}

// tokenReset clears the stored authentication token and the last login time.
func (c *Client) tokenReset() {
	c.tokenMtx.Lock()
	defer c.tokenMtx.Unlock()

	c.token = ""
	c.lastLogin = time.Time{}
}

// tokenSet sets the stored authentication token and sets the last login time
// to the current time.
func (c *Client) tokenSet(token string) {
	c.tokenMtx.Lock()
	defer c.tokenMtx.Unlock()

	c.token = token
	c.lastLogin = time.Now()
}

// tokenRead performs a synchronized read of the stored authentication token.
func (c *Client) tokenRead() string {
	c.tokenMtx.RLock()
	defer c.tokenMtx.RUnlock()

	return c.token
}
