package container

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	mobyclient "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	client, err := GetDockerClient(ctx)
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	defer client.Close()

	dockerBuild := NewDockerBuildExecutor(NewDockerBuildExecutorInput{
		ContextDir: "testdata",
		ImageTag:   "envmergetest",
	})

	err = dockerBuild(ctx)
	assert.NoError(t, err)

	cr := &containerReference{
		cli: client,
		input: &NewContainerInput{
			Image: "envmergetest",
		},
	}
	env := map[string]string{
		"PATH":         "/usr/local/bin:/usr/bin:/usr/sbin:/bin:/sbin",
		"RANDOM_VAR":   "WITH_VALUE",
		"ANOTHER_VAR":  "",
		"CONFLICT_VAR": "I_EXIST_IN_MULTIPLE_PLACES",
	}

	envExecutor := cr.extractFromImageEnv(&env)
	err = envExecutor(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"PATH":            "/usr/local/bin:/usr/bin:/usr/sbin:/bin:/sbin:/this/path/does/not/exists/anywhere:/this/either",
		"RANDOM_VAR":      "WITH_VALUE",
		"ANOTHER_VAR":     "",
		"SOME_RANDOM_VAR": "",
		"ANOTHER_ONE":     "BUT_I_HAVE_VALUE",
		"CONFLICT_VAR":    "I_EXIST_IN_MULTIPLE_PLACES",
	}, env)
}

type mockDockerClient struct {
	mobyclient.APIClient
	mock.Mock
}

func (m *mockDockerClient) ExecCreate(ctx context.Context, id string, opts mobyclient.ExecCreateOptions) (mobyclient.ExecCreateResult, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(mobyclient.ExecCreateResult), args.Error(1)
}

func (m *mockDockerClient) ExecAttach(ctx context.Context, id string, opts mobyclient.ExecAttachOptions) (mobyclient.ExecAttachResult, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(mobyclient.ExecAttachResult), args.Error(1)
}

func (m *mockDockerClient) ExecInspect(ctx context.Context, execID string, opts mobyclient.ExecInspectOptions) (mobyclient.ExecInspectResult, error) {
	args := m.Called(ctx, execID, opts)
	return args.Get(0).(mobyclient.ExecInspectResult), args.Error(1)
}

func (m *mockDockerClient) CopyToContainer(ctx context.Context, id string, options mobyclient.CopyToContainerOptions) (mobyclient.CopyToContainerResult, error) {
	args := m.Called(ctx, id, options)
	return args.Get(0).(mobyclient.CopyToContainerResult), args.Error(1)
}

func (m *mockDockerClient) ContainerKill(ctx context.Context, containerID string, opts mobyclient.ContainerKillOptions) (mobyclient.ContainerKillResult, error) {
	args := m.Called(ctx, containerID, opts)
	return args.Get(0).(mobyclient.ContainerKillResult), args.Error(1)
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options mobyclient.ContainerStartOptions) (mobyclient.ContainerStartResult, error) {
	args := m.Called(ctx, containerID, options)
	return args.Get(0).(mobyclient.ContainerStartResult), args.Error(1)
}

type endlessReader struct {
	io.Reader
}

func (r endlessReader) Read(_ []byte) (n int, err error) {
	time.Sleep(100 * time.Millisecond)
	return 1, nil
}

type mockConn struct {
	net.Conn
	mock.Mock
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockConn) Close() (err error) {
	return nil
}

func TestDockerExecAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &mockConn{}
	conn.On("Write", mock.AnythingOfType("[]uint8")).Return(1, nil)

	client := &mockDockerClient{}
	client.On("ExecCreate", ctx, "123", mock.AnythingOfType("client.ExecCreateOptions")).Return(mobyclient.ExecCreateResult{ID: "id"}, nil)
	attached := make(chan struct{})
	client.On("ExecAttach", ctx, "id", mock.AnythingOfType("client.ExecAttachOptions")).Run(func(_ mock.Arguments) {
		close(attached)
	}).Return(mobyclient.ExecAttachResult{
		HijackedResponse: mobyclient.HijackedResponse{
			Conn:   conn,
			Reader: bufio.NewReader(endlessReader{}),
		},
	}, nil)
	client.On("ContainerKill", mock.Anything, "123", mock.MatchedBy(func(opts mobyclient.ContainerKillOptions) bool {
		return opts.Signal == "kill"
	})).Run(func(_ mock.Arguments) {
		<-attached
	}).Return(mobyclient.ContainerKillResult{}, nil)
	client.On("ContainerStart", mock.Anything, "123", mock.AnythingOfType("client.ContainerStartOptions")).Return(mobyclient.ContainerStartResult{}, nil)

	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	channel := make(chan error)

	go func() {
		channel <- cr.execExt([]string{""}, map[string]string{}, "user", "workdir")(ctx)
	}()

	cancel()

	err := <-channel
	assert.ErrorIs(t, err, context.Canceled)

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestDockerExecFailure(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	client := &mockDockerClient{}
	client.On("ExecCreate", ctx, "123", mock.AnythingOfType("client.ExecCreateOptions")).Return(mobyclient.ExecCreateResult{ID: "id"}, nil)
	client.On("ExecAttach", ctx, "id", mock.AnythingOfType("client.ExecAttachOptions")).Return(mobyclient.ExecAttachResult{
		HijackedResponse: mobyclient.HijackedResponse{
			Conn:   conn,
			Reader: bufio.NewReader(strings.NewReader("output")),
		},
	}, nil)
	client.On("ExecInspect", ctx, "id", mobyclient.ExecInspectOptions{}).Return(mobyclient.ExecInspectResult{
		ExitCode: 1,
	}, nil)

	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.execExt([]string{""}, map[string]string{}, "user", "workdir")(ctx)
	assert.Error(t, err, "exit with `FAILURE`: 1")

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestDockerCopyTarStream(t *testing.T) {
	ctx := context.Background()

	client := &mockDockerClient{}
	client.On("CopyToContainer", ctx, "123", mock.MatchedBy(func(opts mobyclient.CopyToContainerOptions) bool {
		return opts.DestinationPath == "/" && opts.Content != nil
	})).Return(mobyclient.CopyToContainerResult{}, nil)
	client.On("CopyToContainer", ctx, "123", mock.MatchedBy(func(opts mobyclient.CopyToContainerOptions) bool {
		return opts.DestinationPath == "/var/run/act" && opts.Content != nil
	})).Return(mobyclient.CopyToContainerResult{}, nil)
	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	_ = cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})

	client.AssertExpectations(t)
}

func TestDockerCopyTarStreamErrorInCopyFiles(t *testing.T) {
	ctx := context.Background()

	merr := fmt.Errorf("failure")

	client := &mockDockerClient{}
	client.On("CopyToContainer", ctx, "123", mock.MatchedBy(func(opts mobyclient.CopyToContainerOptions) bool {
		return opts.DestinationPath == "/" && opts.Content != nil
	})).Return(mobyclient.CopyToContainerResult{}, merr)
	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})
	assert.ErrorIs(t, err, merr)

	client.AssertExpectations(t)
}

func TestDockerCopyTarStreamErrorInMkdir(t *testing.T) {
	ctx := context.Background()

	merr := fmt.Errorf("failure")

	client := &mockDockerClient{}
	client.On("CopyToContainer", ctx, "123", mock.MatchedBy(func(opts mobyclient.CopyToContainerOptions) bool {
		return opts.DestinationPath == "/" && opts.Content != nil
	})).Return(mobyclient.CopyToContainerResult{}, nil)
	client.On("CopyToContainer", ctx, "123", mock.MatchedBy(func(opts mobyclient.CopyToContainerOptions) bool {
		return opts.DestinationPath == "/var/run/act" && opts.Content != nil
	})).Return(mobyclient.CopyToContainerResult{}, merr)
	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})
	assert.ErrorIs(t, err, merr)

	client.AssertExpectations(t)
}

// Type assert containerReference implements ExecutionsEnvironment
var _ ExecutionsEnvironment = &containerReference{}
