package core

import (
	"context"
	"fmt"
	"log"
	"os"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
)

type Core struct {
	ctx      context.Context
	cancel   context.CancelFunc
	instance *box.Box
	opts     Options
}

type Options struct {
	ConfigPath string
}

func NewCore(opts Options) *Core {
	ctx, cancel := context.WithCancel(context.Background())
	return &Core{
		ctx:    ctx,
		cancel: cancel,
		opts:   opts,
	}
}

func (c *Core) Start() error {
	data, err := os.ReadFile(c.opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var opt option.Options
	ctx := box.Context(
		context.Background(),
		include.InboundRegistry(),
		include.OutboundRegistry(),
		include.EndpointRegistry(),
	)
	if err := opt.UnmarshalJSONContext(ctx, data); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return c.startInstance(opt)
}

func (c *Core) StartWithOptions(opt option.Options) error {
	return c.startInstance(opt)
}

func (c *Core) startInstance(opt option.Options) error {
	ctx := box.Context(
		context.Background(),
		include.InboundRegistry(),
		include.OutboundRegistry(),
		include.EndpointRegistry(),
	)

	instance, err := box.New(box.Options{
		Context: ctx,
		Options: opt,
	})
	if err != nil {
		return fmt.Errorf("create box: %w", err)
	}
	c.instance = instance

	if err := instance.Start(); err != nil {
		return fmt.Errorf("start box: %w", err)
	}

	log.Println("VPN core started")
	return nil
}

func (c *Core) Close() error {
	c.cancel()
	if c.instance != nil {
		return c.instance.Close()
	}
	return nil
}
