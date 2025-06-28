package tr

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/hekmon/transmissionrpc/v2"
)

type Client struct {
	rpc *transmissionrpc.Client
}

func New(url, user, pass string) (*Client, error) {
	rpc, err := transmissionrpc.New(url, user, pass, nil)
	if err != nil {
		return nil, err
	}
	return &Client{rpc: rpc}, nil
}

func (c *Client) AddMagnet(ctx context.Context, magnet, dir string) (int64, error) {
	t, err := c.rpc.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
		Filename:    &magnet,
		DownloadDir: &dir,
	})
	if err != nil {
		return 0, err
	}
	return *t.ID, nil
}

func (c *Client) AddTorrentFile(ctx context.Context, raw []byte, dir string) (int64, error) {
	meta := base64.StdEncoding.EncodeToString(raw)
	t, err := c.rpc.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
		MetaInfo:    &meta,
		DownloadDir: &dir,
	})
	if err != nil {
		return 0, err
	}
	return *t.ID, nil
}

func (c *Client) IsComplete(ctx context.Context, id int64) (bool, error) {
	fields := []string{"percentDone", "isFinished"}
	ts, err := c.rpc.TorrentGet(ctx, fields, []int64{id})
	if err != nil {
		return false, err
	}
	if len(ts) == 0 {
		return false, errors.New("torrent not found")
	}
	t := ts[0]
	return (t.IsFinished != nil && *t.IsFinished) ||
		(t.PercentDone != nil && *t.PercentDone >= 0.999), nil
}
