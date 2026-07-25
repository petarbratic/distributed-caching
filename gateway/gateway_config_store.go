package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	gatewayConfigKey        = "gateway:config:data"
	gatewayConfigVersionKey = "gateway:config:version"
)

type GatewayConfigStore struct {
	redis *redis.Client
}

func (s *GatewayConfigStore) Initialize(
	ctx context.Context,
	defaultConfig GatewayConfig,
) error {
	data, err := json.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("marshal default gateway config: %w", err)
	}

	script := redis.NewScript(`
		if redis.call("EXISTS", KEYS[1]) == 0 then
			redis.call("SET", KEYS[1], ARGV[1])
			redis.call("SET", KEYS[2], 1)
			return 1
		end

		if redis.call("EXISTS", KEYS[2]) == 0 then
			redis.call("SET", KEYS[2], 1)
		end

		return 0
	`)

	if _, err := script.Run(
		ctx,
		s.redis,
		[]string{
			gatewayConfigKey,
			gatewayConfigVersionKey,
		},
		data,
	).Result(); err != nil {
		return fmt.Errorf("initialize gateway config: %w", err)
	}

	return nil
}

func (s *GatewayConfigStore) Get(
	ctx context.Context,
) (GatewayConfig, int64, error) {
	values, err := s.redis.MGet(
		ctx,
		gatewayConfigKey,
		gatewayConfigVersionKey,
	).Result()
	if err != nil {
		return GatewayConfig{}, 0, fmt.Errorf(
			"read gateway config: %w",
			err,
		)
	}

	if len(values) != 2 || values[0] == nil || values[1] == nil {
		return GatewayConfig{}, 0, fmt.Errorf(
			"gateway configuration is not initialized",
		)
	}

	configJSON, ok := values[0].(string)
	if !ok {
		return GatewayConfig{}, 0, fmt.Errorf(
			"invalid gateway configuration value",
		)
	}

	versionValue, ok := values[1].(string)
	if !ok {
		return GatewayConfig{}, 0, fmt.Errorf(
			"invalid gateway configuration version",
		)
	}

	var config GatewayConfig

	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return GatewayConfig{}, 0, fmt.Errorf(
			"decode gateway config: %w",
			err,
		)
	}

	version, err := strconv.ParseInt(versionValue, 10, 64)
	if err != nil {
		return GatewayConfig{}, 0, fmt.Errorf(
			"parse gateway config version: %w",
			err,
		)
	}

	return config, version, nil
}

func (s *GatewayConfigStore) Update(
	ctx context.Context,
	config GatewayConfig,
) (int64, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return 0, fmt.Errorf("marshal gateway config: %w", err)
	}

	var versionCommand *redis.IntCmd

	_, err = s.redis.TxPipelined(
		ctx,
		func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, gatewayConfigKey, data, 0)
			versionCommand = pipe.Incr(
				ctx,
				gatewayConfigVersionKey,
			)

			return nil
		},
	)
	if err != nil {
		return 0, fmt.Errorf("update gateway config: %w", err)
	}

	version, err := versionCommand.Result()
	if err != nil {
		return 0, fmt.Errorf(
			"read updated gateway config version: %w",
			err,
		)
	}

	return version, nil
}
