package module_test

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/realDragonium/Ultraviolet/mc"
	"github.com/realDragonium/Ultraviolet/module"
)

type statusCacheConnCreator struct {
	conn net.Conn
	err  error
}

func (creator statusCacheConnCreator) Conn() func() (net.Conn, error) {
	return func() (net.Conn, error) {
		return creator.conn, creator.err
	}
}

type statusCacheConnCreatorMultipleCalls struct {
	connCh <-chan net.Conn
}

func (creator statusCacheConnCreatorMultipleCalls) Conn() func() (net.Conn, error) {
	conn := <-creator.connCh
	return func() (net.Conn, error) {
		return conn, nil
	}
}

func statusCall_TestError(t *testing.T, cache *module.StatusCache, errCh chan error) mc.Packet {
	t.Helper()
	answerCh := make(chan mc.Packet)
	go func() {
		answer, err := (*cache).Status()
		if err != nil {
			errCh <- err
			return
		}
		answerCh <- answer
	}()

	select {
	case answer := <-answerCh:
		t.Log("worker has successfully responded")
		return answer
	case err := <-errCh:
		t.Fatalf("didnt expect an error but got: %v", err)
	}
	return mc.Packet{}
}

type serverSimulator struct {
	callAmount      int
	closeConnByStep int
}

func (simulator *serverSimulator) simulateServerStatus(conn net.Conn, statusPacket mc.Packet) error {
	simulator.callAmount++
	mcConn := mc.NewMcConn(conn)
	if simulator.closeConnByStep == 1 {
		return conn.Close()
	}
	_, err := mcConn.ReadPacket()
	if err != nil {
		return err
	}
	if simulator.closeConnByStep == 2 {
		return conn.Close()
	}
	_, err = mcConn.ReadPacket()
	if err != nil {
		return err
	}
	if simulator.closeConnByStep == 3 {
		return conn.Close()
	}
	err = mcConn.WritePacket(statusPacket)
	if err != nil {
		return err
	}
	if simulator.closeConnByStep == 4 {
		return conn.Close()
	}
	pingPk, err := mcConn.ReadPacket()
	if err != nil {
		return err
	}
	time.Sleep(defaultChTimeout / 10) // '/ 10' part just so its shorter than the time.After later
	if simulator.closeConnByStep == 5 {
		return conn.Close()
	}
	err = mcConn.WritePacket(pingPk)
	if err != nil {
		return err
	}

	return nil
}

func TestStatusCache(t *testing.T) {
	protocolVersion := 755
	cooldown := time.Minute
	statusPacket := mc.SimpleStatus{
		Name:        "backend",
		Protocol:    protocolVersion,
		Description: "some random motd text",
	}.Marshal()

	t.Run("normal flow", func(t *testing.T) {
		errCh := make(chan error)
		answerCh := make(chan mc.Packet)
		c1, c2 := net.Pipe()
		connCreator := statusCacheConnCreator{conn: c1}
		statusCache := module.NewStatusCache(protocolVersion, cooldown, connCreator)
		simulator := serverSimulator{}
		go func() {
			err := simulator.simulateServerStatus(c2, statusPacket)
			if err != nil {
				errCh <- err
			}
		}()
		go func() {
			answer, err := statusCache.Status()
			if err != nil {
				errCh <- err
			}
			answerCh <- answer
		}()

		var answer mc.Packet

		select {
		case answer = <-answerCh:
			t.Log("worker has successfully responded")
		case err := <-errCh:
			t.Fatalf("didnt expect an error but got: %v", err)
		case <-time.After(defaultChTimeout):
			t.Fatal("timed out")
		}

		if !cmp.Equal(statusPacket, answer) {
			t.Error("received different packet than we expected!")
			t.Logf("expected: %#v", statusPacket)
			t.Logf("received: %#v", answer)
		}
		if simulator.callAmount != 1 {
			t.Errorf("expected backend to be called 1 time but got called %v time(s)", simulator.callAmount)
		}
	})

	t.Run("doesnt call again while in cooldown", func(t *testing.T) {
		errCh := make(chan error)
		connCh := make(chan net.Conn, 1)
		connCreator := &statusCacheConnCreatorMultipleCalls{connCh: connCh}
		statusCache := module.NewStatusCache(protocolVersion, cooldown, connCreator)
		simulator := serverSimulator{}

		c1, c2 := net.Pipe()
		connCh <- c1
		go simulator.simulateServerStatus(c2, statusPacket)
		statusCall_TestError(t, &statusCache, errCh)

		// This will timeout if its going to call a second time
		statusCall_TestError(t, &statusCache, errCh)
		if simulator.callAmount != 1 {
			t.Errorf("expected backend to be called 1 time but got called %v time(s)", simulator.callAmount)
		}
	})

	t.Run("does call again after cooldown", func(t *testing.T) {
		cooldown = time.Microsecond
		errCh := make(chan error)
		connCh := make(chan net.Conn, 1)
		connCreator := &statusCacheConnCreatorMultipleCalls{connCh: connCh}
		statusCache := module.NewStatusCache(protocolVersion, cooldown, connCreator)
		simulator := serverSimulator{}

		c1, c2 := net.Pipe()
		connCh <- c1
		go simulator.simulateServerStatus(c2, statusPacket)
		statusCall_TestError(t, &statusCache, errCh)
		time.Sleep(cooldown)
		c1, c2 = net.Pipe()
		connCh <- c1
		go simulator.simulateServerStatus(c2, statusPacket)
		statusCall_TestError(t, &statusCache, errCh)
		if simulator.callAmount != 2 {
			t.Errorf("expected backend to be called 2 time but got called %v time(s)", simulator.callAmount)
		}
	})

	t.Run("returns with error when connCreator returns error ", func(t *testing.T) {
		t.Run("with conn being nil", func(t *testing.T) {
			usedError := errors.New("cant create connection")
			connCreator := statusCacheConnCreator{err: usedError, conn: nil}
			statusCache := module.NewStatusCache(protocolVersion, cooldown, connCreator)
			_, err := statusCache.Status()
			if !errors.Is(err, usedError) {
				t.Errorf("expected an error but something else: %v", err)
			}
		})
		t.Run("with conn being an connection", func(t *testing.T) {
			usedError := errors.New("cant create connection")
			connCreator := statusCacheConnCreator{err: usedError, conn: &net.TCPConn{}}
			statusCache := module.NewStatusCache(protocolVersion, cooldown, connCreator)
			_, err := statusCache.Status()
			if !errors.Is(err, usedError) {
				t.Errorf("expected an error but something else: %v", err)
			}
		})
	})

	t.Run("test closing connection early", func(t *testing.T) {
		tt := []struct {
			matchStatus       bool
			shouldReturnError bool
			closeConnByStep   int
		}{
			{
				matchStatus:       false,
				shouldReturnError: true,
				closeConnByStep:   1,
			},
			{
				matchStatus:       false,
				shouldReturnError: true,
				closeConnByStep:   2,
			},
			{
				matchStatus:       false,
				shouldReturnError: true,
				closeConnByStep:   3,
			},
			{
				matchStatus:       true,
				shouldReturnError: false,
				closeConnByStep:   4,
			},
			{
				matchStatus:       true,
				shouldReturnError: false,
				closeConnByStep:   5,
			},
		}

		for _, tc := range tt {
			name := fmt.Sprintf("closeConnBy:%v", tc.closeConnByStep)
			t.Run(name, func(t *testing.T) {
				errCh := make(chan error)
				answerCh := make(chan mc.Packet)

				c1, c2 := net.Pipe()
				connCreator := statusCacheConnCreator{conn: c1}
				statusCache := module.NewStatusCache(protocolVersion, cooldown, connCreator)
				simulator := serverSimulator{
					closeConnByStep: tc.closeConnByStep,
				}
				go func() {
					err := simulator.simulateServerStatus(c2, statusPacket)
					if err != nil {
						errCh <- err
					}
				}()
				closeCh := make(chan struct{})
				go func() {
					answer, err := statusCache.Status()
					if err != nil {
						errCh <- err
					}
					select {
					case answerCh <- answer:
					case <-closeCh:
					}

				}()

				var answer mc.Packet
				var err error
				select {
				case answer = <-answerCh:
					t.Log("worker has successfully responded")
				case err = <-errCh:
					closeCh <- struct{}{}
					if !tc.shouldReturnError {
						t.Fatalf("didnt expect an error but got: %v", err)
					}
				case <-time.After(defaultChTimeout):
					closeCh <- struct{}{}
					t.Fatal("timed out")
				}

				if err == nil && tc.shouldReturnError {
					t.Fatal("expected an error but got nothing")
				}

				if tc.matchStatus && !cmp.Equal(statusPacket, answer) {
					t.Error("received different packet than we expected!")
					t.Logf("expected: %v", statusPacket)
					t.Logf("received: %v", answer)
				}

			})
		}
	})
}
