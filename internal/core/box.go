package core

import (
    "context"
    "encoding/json"
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
    f, err := os.Open(c.opts.ConfigPath)
    if err != nil {
        return fmt.Errorf("open config: %w", err)
    }
    defer f.Close()

    var opt option.Options
    if err := json.NewDecoder(f).Decode(&opt); err != nil {
        return fmt.Errorf("parse config: %w", err)
    }

    // Инициализируем контекст со всеми необходимыми регистрами для sing-box v1.11.0
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

    log.Println("VPN core started successfully")
    return nil
}

func (c *Core) StartWithOptions(opt option.Options) error {
    // Инициализируем контекст со всеми необходимыми регистрами для sing-box v1.11.0
    ctx := box.Context(
        context.Background(),
        include.InboundRegistry(),
        include.OutboundRegistry(),
        include.EndpointRegistry(),
    )
    // Debug: покажем, как выглядят опции после парсинга
    if b, err := json.MarshalIndent(opt, "", "  "); err == nil {
        fmt.Fprintf(os.Stderr, "Parsed option.Options: %s\n", string(b))
    } else {
        fmt.Fprintf(os.Stderr, "Failed to marshal opt: %v\n", err)
    }

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

    log.Println("VPN core started successfully")
    return nil
}

func (c *Core) Close() error {
    c.cancel()
    if c.instance != nil {
        return c.instance.Close()
    }
    return nil
}

func (c *Core) Wait() {
    <-c.ctx.Done()
}
