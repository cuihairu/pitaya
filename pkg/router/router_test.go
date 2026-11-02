package router

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/pitaya/v3/pkg/cluster"
	"github.com/topfreegames/pitaya/v3/pkg/cluster/mocks"
	"github.com/topfreegames/pitaya/v3/pkg/conn/message"
	"github.com/topfreegames/pitaya/v3/pkg/constants"
	"github.com/topfreegames/pitaya/v3/pkg/protos"
	"github.com/topfreegames/pitaya/v3/pkg/route"
)

var (
	serverID   = "id"
	serverType = "serverType"
	frontend   = true
	server     = cluster.NewServer(serverID, serverType, frontend)
	servers    = map[string]*cluster.Server{
		serverID: server,
	}

	routingFunction = func(
		ctx context.Context,
		route *route.Route,
		payload []byte,
		servers map[string]*cluster.Server,
	) (*cluster.Server, error) {
		return server, nil
	}
)

var routerTables = map[string]struct {
	server     *cluster.Server
	serverType string
	rpcType    protos.RPCType
	err        error
}{
	"test_server_has_route_func":   {server, serverType, protos.RPCType_Sys, nil},
	"test_server_use_default_func": {server, "notRegisteredType", protos.RPCType_Sys, nil},
	"test_user_use_default_func":   {server, serverType, protos.RPCType_User, nil},
	"test_error_on_service_disc":   {nil, serverType, protos.RPCType_Sys, errors.New("sd error")},
}

var addRouteRouterTables = map[string]struct {
	serverType string
}{
	"test_overrige_server_type": {serverType},
	"test_new_server_type":      {"notRegisteredType"},
}

func TestNew(t *testing.T) {
	t.Parallel()
	router := New()
	assert.NotNil(t, router)
}

func TestDefaultRoute(t *testing.T) {
	t.Parallel()

	router := New()

	retServer := router.defaultRoute(servers)
	assert.Equal(t, server, retServer)
}

func TestRoute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	route := route.NewRoute(serverType, "service", "method")

	for name, table := range routerTables {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockServiceDiscovery := mocks.NewMockServiceDiscovery(ctrl)
			mockServiceDiscovery.EXPECT().
				GetServersByType(table.serverType).
				Return(servers, table.err)

			router := New()
			router.AddRoute(serverType, routingFunction)
			router.SetServiceDiscovery(mockServiceDiscovery)

			retServer, err := router.Route(ctx, table.rpcType, table.serverType, route, &message.Message{
				Data: []byte{0x01},
			})
			assert.Equal(t, table.server, retServer)
			assert.Equal(t, table.err, err)
		})
	}
}

func TestAddRoute(t *testing.T) {
	t.Parallel()

	for name, table := range addRouteRouterTables {
		t.Run(name, func(t *testing.T) {
			router := New()
			router.AddRoute(table.serverType, routingFunction)

			assert.NotNil(t, router.routesMap[table.serverType])
			assert.Nil(t, router.routesMap["anotherServerType"])
		})
	}
}

func TestRouteFailIfNullServiceDiscovery(t *testing.T) {
	t.Parallel()

	router := New()
	_, err := router.Route(context.Background(), protos.RPCType_Sys, serverType, route.NewRoute(serverType, "service", "method"), &message.Message{
		Data: []byte{0x01},
	})
	assert.Equal(t, constants.ErrServiceDiscoveryNotInitialized, err)
}

func TestDefaultRouteEmptyServers(t *testing.T) {
	t.Parallel()

	router := New()
	assert.Nil(t, router.defaultRoute(map[string]*cluster.Server{}))
}

func TestRouteNoServersAvailable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rt := route.NewRoute(serverType, "service", "method")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockServiceDiscovery := mocks.NewMockServiceDiscovery(ctrl)
	mockServiceDiscovery.EXPECT().
		GetServersByType(serverType).
		Return(map[string]*cluster.Server{}, nil)
	router := New()
	router.SetServiceDiscovery(mockServiceDiscovery)
	_, err := router.Route(ctx, protos.RPCType_User, serverType, rt, &message.Message{Data: []byte{0x01}})
	assert.Equal(t, constants.ErrNoServersAvailableOfType, err)
}

func TestRouteCustomFuncNilServer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rt := route.NewRoute(serverType, "service", "method")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockServiceDiscovery := mocks.NewMockServiceDiscovery(ctrl)
	mockServiceDiscovery.EXPECT().
		GetServersByType(serverType).
		Return(map[string]*cluster.Server{}, nil)

	router := New()
	// A custom route function may return (nil, nil) when serversOfType is
	// empty; Route must surface ErrNoServersAvailableOfType instead of
	// letting remoteCall pass a nil target to rpcClient.Call.
	router.AddRoute(serverType, func(
		ctx context.Context,
		rt *route.Route,
		payload []byte,
		servers map[string]*cluster.Server,
	) (*cluster.Server, error) {
		return nil, nil
	})
	router.SetServiceDiscovery(mockServiceDiscovery)

	_, err := router.Route(ctx, protos.RPCType_Sys, serverType, rt, &message.Message{Data: []byte{0x01}})
	assert.Equal(t, constants.ErrNoServersAvailableOfType, err)
}
