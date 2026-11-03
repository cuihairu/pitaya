package client

import (
	"math"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/pitaya/v3/pkg/conn/message"
	"github.com/topfreegames/pitaya/v3/pkg/helpers"
	"github.com/topfreegames/pitaya/v3/pkg/mocks"
)

func TestSendRequestShouldTimeout(t *testing.T) {
	c := New(logrus.InfoLevel, 100*time.Millisecond)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockPlayerConn(ctrl)
	c.conn = mockConn
	go c.pendingRequestsReaper()

	route := "com.sometest.route"
	data := []byte{0x02, 0x03, 0x04}

	m := message.Message{
		Type:  message.Request,
		ID:    1,
		Route: route,
		Data:  data,
		Err:   false,
	}

	pkt, err := c.buildPacket(m)
	assert.NoError(t, err)

	mockConn.EXPECT().Write(pkt)

	c.IncomingMsgChan = make(chan *message.Message, 10)

	c.nextID = 0
	c.SendRequest(route, data)

	msg := helpers.ShouldEventuallyReceive(t, c.IncomingMsgChan, 2*time.Second).(*message.Message)

	assert.Equal(t, true, msg.Err)
}

func TestSendHeartbeatsNonPositiveInterval(t *testing.T) {
	c := &Client{}
	// a non-positive heartbeat interval must be a no-op, not a panic in
	// time.NewTicker
	c.sendHeartbeats(0)
	c.sendHeartbeats(-1)
}

func TestSendRequestIDWraparoundSkipsZero(t *testing.T) {
	c := New(logrus.InfoLevel)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockPlayerConn(ctrl)
	mockConn.EXPECT().Write(gomock.Any()).Return(16, nil).Times(2)
	c.conn = mockConn
	c.closeChan = make(chan struct{})

	// Preload the counter so the next increment wraps to 0, the sentinel
	// "notify" ID. sendMsg must detect this and skip to 1.
	c.nextID = math.MaxUint64

	id, err := c.sendMsg(message.Request, "test.route", []byte{0x01})
	assert.NoError(t, err)
	assert.NotEqual(t, uint64(0), id, "request ID must never be 0")
	assert.Equal(t, uint64(1), id)

	id, err = c.sendMsg(message.Request, "test.route", []byte{0x02})
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), id)
}
