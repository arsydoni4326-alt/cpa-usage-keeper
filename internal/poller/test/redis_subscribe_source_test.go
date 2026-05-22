package poller_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"cpa-usage-keeper/internal/poller"
)

func TestRedisSubscribeSourceRejectsSubscribeError(t *testing.T) {
	server := newRESPServer(t, func(t *testing.T, conn net.Conn) {
		reader := bufio.NewReader(conn)
		if got := readRESPCommandForPollerTest(t, reader); strings.Join(got, " ") != "AUTH secret" {
			t.Fatalf("unexpected auth command: %v", got)
		}
		fmt.Fprint(conn, "+OK\r\n")
		if got := readRESPCommandForPollerTest(t, reader); strings.Join(got, " ") != "SUBSCRIBE usage" {
			t.Fatalf("unexpected subscribe command: %v", got)
		}
		fmt.Fprint(conn, "-ERR subscribe disabled\r\n")
	})

	source := poller.NewRedisSubscribeSource(poller.RedisSubscribeOptions{RedisAddr: server.addr, ManagementKey: "secret", Timeout: time.Second})
	_, err := source.Subscribe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "subscribe disabled") {
		t.Fatalf("expected subscribe error, got %v", err)
	}
}

func TestRedisSubscriptionReceiveHonorsContextCancel(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	server := newRESPServer(t, func(t *testing.T, conn net.Conn) {
		reader := bufio.NewReader(conn)
		readRESPCommandForPollerTest(t, reader)
		fmt.Fprint(conn, "+OK\r\n")
		readRESPCommandForPollerTest(t, reader)
		fmt.Fprint(conn, "*3\r\n$9\r\nsubscribe\r\n$5\r\nusage\r\n:1\r\n")
		<-done
	})

	source := poller.NewRedisSubscribeSource(poller.RedisSubscribeOptions{RedisAddr: server.addr, ManagementKey: "secret", Timeout: time.Second})
	sub, err := source.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer sub.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := sub.Receive(ctx)
		errCh <- err
	}()
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canceled receive")
	}
}

func TestRedisSubscriptionReceiveHonorsDeadlineContextCancel(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	server := newRESPServer(t, func(t *testing.T, conn net.Conn) {
		reader := bufio.NewReader(conn)
		readRESPCommandForPollerTest(t, reader)
		fmt.Fprint(conn, "+OK\r\n")
		readRESPCommandForPollerTest(t, reader)
		fmt.Fprint(conn, "*3\r\n$9\r\nsubscribe\r\n$5\r\nusage\r\n:1\r\n")
		<-done
	})

	source := poller.NewRedisSubscribeSource(poller.RedisSubscribeOptions{RedisAddr: server.addr, ManagementKey: "secret", Timeout: time.Second})
	sub, err := source.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer sub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	errCh := make(chan error, 1)
	go func() {
		_, err := sub.Receive(ctx)
		errCh <- err
	}()
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deadline context cancellation")
	}
}

func TestRedisSubscriptionReceiveHonorsContextDeadline(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	server := newRESPServer(t, func(t *testing.T, conn net.Conn) {
		reader := bufio.NewReader(conn)
		readRESPCommandForPollerTest(t, reader)
		fmt.Fprint(conn, "+OK\r\n")
		readRESPCommandForPollerTest(t, reader)
		fmt.Fprint(conn, "*3\r\n$9\r\nsubscribe\r\n$5\r\nusage\r\n:1\r\n")
		fmt.Fprint(conn, "*3\r\n$7\r\nmessage\r\n$5\r\nusage\r\n$18\r\n{\"request_id\":\"x\"}\r\n")
		<-done
	})

	source := poller.NewRedisSubscribeSource(poller.RedisSubscribeOptions{RedisAddr: server.addr, ManagementKey: "secret", Timeout: time.Second})
	sub, err := source.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer sub.Close()

	message, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("first Receive returned error: %v", err)
	}
	if message != `{"request_id":"x"}` {
		t.Fatalf("unexpected message %q", message)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = sub.Receive(ctx)
	if err == nil || ctx.Err() == nil {
		t.Fatalf("expected receive to honor context deadline, got %v", err)
	}
}

type respTestServer struct {
	addr string
	done chan struct{}
}

func newRESPServer(t *testing.T, handler func(*testing.T, net.Conn)) respTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := respTestServer{addr: listener.Addr().String(), done: make(chan struct{})}
	t.Cleanup(func() {
		close(server.done)
		listener.Close()
	})
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(t, conn)
	}()
	t.Cleanup(func() { <-accepted })
	return server
}

func readRESPCommandForPollerTest(t *testing.T, reader *bufio.Reader) []string {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read command header: %v", err)
	}
	var count int
	if _, err := fmt.Sscanf(line, "*%d\r\n", &count); err != nil {
		t.Fatalf("parse command header %q: %v", line, err)
	}
	parts := make([]string, 0, count)
	for range count {
		bulkHeader, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read bulk header: %v", err)
		}
		var size int
		if _, err := fmt.Sscanf(bulkHeader, "$%d\r\n", &size); err != nil {
			t.Fatalf("parse bulk header %q: %v", bulkHeader, err)
		}
		buf := make([]byte, size+2)
		if _, err := reader.Read(buf); err != nil {
			t.Fatalf("read bulk body: %v", err)
		}
		parts = append(parts, string(buf[:size]))
	}
	return parts
}
