//go:build integration

/*
 * Copyright 2022 The Yorkie Authors. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package integration

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yorkie-team/yorkie/admin"
	"github.com/yorkie-team/yorkie/api/types"
	"github.com/yorkie-team/yorkie/client"
	"github.com/yorkie-team/yorkie/pkg/document"
	"github.com/yorkie-team/yorkie/pkg/document/key"
	"github.com/yorkie-team/yorkie/pkg/document/proxy"
	"github.com/yorkie-team/yorkie/server"
	"github.com/yorkie-team/yorkie/server/logging"
	"github.com/yorkie-team/yorkie/test/helper"
)

func TestHistory(t *testing.T) {
	clients := activeClients(t, 1)
	cli := clients[0]
	defer cleanupClients(t, clients)

	adminCli, err := admin.Dial(defaultServer.AdminAddr())
	assert.NoError(t, err)
	defer func() { assert.NoError(t, adminCli.Close()) }()

	t.Run("history test", func(t *testing.T) {
		ctx := context.Background()

		d1 := document.New(key.Key(t.Name()))
		assert.NoError(t, cli.Attach(ctx, d1))
		defer func() { assert.NoError(t, cli.Detach(ctx, d1)) }()

		assert.NoError(t, d1.Update(func(root *proxy.ObjectProxy) error {
			root.SetNewArray("todos")
			return nil
		}, "create todos"))
		assert.Equal(t, `{"todos":[]}`, d1.Marshal())

		assert.NoError(t, d1.Update(func(root *proxy.ObjectProxy) error {
			root.GetArray("todos").AddString("buy coffee")
			return nil
		}, "buy coffee"))
		assert.Equal(t, `{"todos":["buy coffee"]}`, d1.Marshal())

		assert.NoError(t, d1.Update(func(root *proxy.ObjectProxy) error {
			root.GetArray("todos").AddString("buy bread")
			return nil
		}, "buy bread"))
		assert.Equal(t, `{"todos":["buy coffee","buy bread"]}`, d1.Marshal())
		assert.NoError(t, cli.Sync(ctx))

		changes, err := adminCli.ListChangeSummaries(ctx, "default", d1.Key(), 0, 0, true)
		assert.NoError(t, err)
		assert.Len(t, changes, 3)

		assert.Equal(t, "create todos", changes[2].Message)
		assert.Equal(t, "buy coffee", changes[1].Message)
		assert.Equal(t, "buy bread", changes[0].Message)

		assert.Equal(t, `{"todos":[]}`, changes[2].Snapshot)
		assert.Equal(t, `{"todos":["buy coffee"]}`, changes[1].Snapshot)
		assert.Equal(t, `{"todos":["buy coffee","buy bread"]}`, changes[0].Snapshot)
	})

	t.Run("history test with purging changes", func(t *testing.T) {
		serverConfig := helper.TestConfig()
		serverConfig.Backend.SnapshotWithPurgingChanges = true
		testServer, err := server.New(serverConfig)
		if err != nil {
			log.Fatal(err)
		}

		if err := testServer.Start(); err != nil {
			logging.DefaultLogger().Fatal(err)
		}

		cli2, err := client.Dial(
			testServer.RPCAddr(),
			client.WithPresence(types.Presence{"name": fmt.Sprintf("name-%d", 0)}),
		)
		assert.NoError(t, err)

		err = cli2.Activate(context.Background())
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, cli2.Deactivate(context.Background()))
			assert.NoError(t, cli2.Close())
		}()

		adminCli2, err := admin.Dial(testServer.AdminAddr())
		assert.NoError(t, err)
		defer func() { assert.NoError(t, adminCli2.Close()) }()

		ctx := context.Background()

		d1 := document.New(key.Key(t.Name()))
		assert.NoError(t, cli2.Attach(ctx, d1))
		defer func() { assert.NoError(t, cli2.Detach(ctx, d1)) }()

		assert.NoError(t, d1.Update(func(root *proxy.ObjectProxy) error {
			root.SetNewArray("todos")
			return nil
		}, "create todos"))
		assert.Equal(t, `{"todos":[]}`, d1.Marshal())

		assert.NoError(t, d1.Update(func(root *proxy.ObjectProxy) error {
			root.GetArray("todos").AddString("buy coffee")
			return nil
		}, "buy coffee"))
		assert.Equal(t, `{"todos":["buy coffee"]}`, d1.Marshal())

		assert.NoError(t, d1.Update(func(root *proxy.ObjectProxy) error {
			root.GetArray("todos").AddString("buy bread")
			return nil
		}, "buy bread"))
		assert.Equal(t, `{"todos":["buy coffee","buy bread"]}`, d1.Marshal())
		assert.NoError(t, cli2.Sync(ctx))

		changes, err := adminCli2.ListChangeSummaries(ctx, "default", d1.Key(), 0, 0, true)
		assert.NoError(t, err)
		assert.Len(t, changes, 0)
	})
}
