package redis

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type Client struct {
    *redis.Client
}

func NewClient(redisURL string) *Client {
    opt, err := redis.ParseURL(redisURL)
    if err != nil {
        opt = &redis.Options{
            Addr: redisURL,
        }
    }

    client := redis.NewClient(opt)
    
    return &Client{client}
}

func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    return c.Set(ctx, key, data, expiration).Err()
}

func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) error {
    data, err := c.Get(ctx, key).Result()
    if err != nil {
        return err
    }
    
    return json.Unmarshal([]byte(data), dest)
}

func (c *Client) CacheDomainHealth(ctx context.Context, domainID string, health interface{}) error {
    key := fmt.Sprintf("domain:health:%s", domainID)
    return c.SetJSON(ctx, key, health, 5*time.Minute)
}

func (c *Client) GetCachedDomainHealth(ctx context.Context, domainID string, dest interface{}) error {
    key := fmt.Sprintf("domain:health:%s", domainID)
    return c.GetJSON(ctx, key, dest)
}