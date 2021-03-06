/*
 * Copyright (c) 2014 IBM Corp.
 *
 * All rights reserved. This program and the accompanying materials
 * are made available under the terms of the Eclipse Public License v1.0
 * which accompanies this distribution, and is available at
 * http://www.eclipse.org/legal/epl-v10.html
 *
 * Contributors:
 *    Allan Stockdill-Mander
 */

package mqtt

import (
	"context"
	"sync"

	"github.com/zenreach/paho.mqtt.golang/packets"
)

//PacketAndToken is a struct that contains both a ControlPacket and a
//Token. This struct is passed via channels between the client interface
//code and the underlying code responsible for sending and receiving
//MQTT messages.
type PacketAndToken struct {
	p packets.ControlPacket
	t Token
}

//Token defines the interface for the tokens used to indicate when
//actions have completed.
type Token interface {
	Wait() bool
	WaitContext(context.Context) bool
	flowComplete()
	Error() error
}

type baseToken struct {
	m        sync.RWMutex
	complete chan struct{}
	ready    bool
	err      error
}

// Wait will wait indefinitely for the Token to complete, ie the Publish
// to be sent and confirmed receipt from the broker
func (b *baseToken) Wait() bool {
	b.m.Lock()
	defer b.m.Unlock()
	if !b.ready {
		<-b.complete
		b.ready = true
	}
	return b.ready
}

// WaitContext waits for the flow associated with the Token to complete or the
// context to be cancelled, returns true if it returned before cancellation or
// returns false if the context is cancelled. In the case of cancellation the
// Token does not have an error set in case the caller wishes to wait again
func (b *baseToken) WaitContext(ctx context.Context) bool {
	b.m.Lock()
	defer b.m.Unlock()
	if !b.ready {
		select {
		case <-b.complete:
			b.ready = true
		case <-ctx.Done():
		}
	}
	return b.ready
}

func (b *baseToken) flowComplete() {
	close(b.complete)
}

func (b *baseToken) Error() error {
	b.m.RLock()
	defer b.m.RUnlock()
	return b.err
}

func newToken(tType byte) Token {
	switch tType {
	case packets.Connect:
		return &ConnectToken{baseToken: baseToken{complete: make(chan struct{})}}
	case packets.Subscribe:
		return &SubscribeToken{baseToken: baseToken{complete: make(chan struct{})}, subResult: make(map[string]byte)}
	case packets.Publish:
		return &PublishToken{baseToken: baseToken{complete: make(chan struct{})}}
	case packets.Unsubscribe:
		return &UnsubscribeToken{baseToken: baseToken{complete: make(chan struct{})}}
	case packets.Disconnect:
		return &DisconnectToken{baseToken: baseToken{complete: make(chan struct{})}}
	}
	return nil
}

//ConnectToken is an extension of Token containing the extra fields
//required to provide information about calls to Connect()
type ConnectToken struct {
	baseToken
	returnCode byte
}

//ReturnCode returns the acknowlegement code in the connack sent
//in response to a Connect()
func (c *ConnectToken) ReturnCode() byte {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.returnCode
}

//PublishToken is an extension of Token containing the extra fields
//required to provide information about calls to Publish()
type PublishToken struct {
	baseToken
	messageID uint16
}

//MessageID returns the MQTT message ID that was assigned to the
//Publish packet when it was sent to the broker
func (p *PublishToken) MessageID() uint16 {
	return p.messageID
}

//SubscribeToken is an extension of Token containing the extra fields
//required to provide information about calls to Subscribe()
type SubscribeToken struct {
	baseToken
	subs      []string
	subResult map[string]byte
}

//Result returns a map of topics that were subscribed to along with
//the matching return code from the broker. This is either the Qos
//value of the subscription or an error code.
func (s *SubscribeToken) Result() map[string]byte {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.subResult
}

//UnsubscribeToken is an extension of Token containing the extra fields
//required to provide information about calls to Unsubscribe()
type UnsubscribeToken struct {
	baseToken
}

//DisconnectToken is an extension of Token containing the extra fields
//required to provide information about calls to Disconnect()
type DisconnectToken struct {
	baseToken
}
