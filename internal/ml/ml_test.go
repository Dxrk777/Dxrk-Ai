// SPDX-License-Identifier: MIT
package ml

import (
	"testing"
)

func TestBuildPipelineSFT(t *testing.T) {
	cfg := &TrainConfig{
		Method: MethodSFT,
		Model: ModelConfig{
			BaseModel: "Qwen/Qwen2.5-0.5B",
			HubID:     "dxrk/test-model",
		},
		Dataset: DatasetConfig{
			Name: "tatsu-lab/alpaca",
		},
		PushToHub: true,
	}

	plan := BuildPipeline(cfg)
	if len(plan.Prepare) != 1 {
		t.Fatalf("expected 1 prepare step, got %d", len(plan.Prepare))
	}
	if len(plan.Apply) != 2 {
		t.Fatalf("expected 2 apply steps (train+push), got %d", len(plan.Apply))
	}
	if plan.Prepare[0].ID() != "ml:validate-data" {
		t.Errorf("unexpected prepare step: %s", plan.Prepare[0].ID())
	}
	if plan.Apply[0].ID() != "ml:train" {
		t.Errorf("unexpected first apply step: %s", plan.Apply[0].ID())
	}
	if plan.Apply[1].ID() != "ml:push-hub" {
		t.Errorf("unexpected second apply step: %s", plan.Apply[1].ID())
	}
}

func TestBuildPipelineDPO(t *testing.T) {
	cfg := &TrainConfig{
		Method: MethodDPO,
		Model: ModelConfig{
			BaseModel: "Qwen/Qwen2.5-0.5B",
			HubID:     "dxrk/test-dpo",
		},
		Dataset: DatasetConfig{
			Name: "Anthropic/hh-rlhf",
		},
		PushToHub:  false,
		ExportGGUF: true,
	}

	plan := BuildPipeline(cfg)
	// validate + train + convert
	if len(plan.Prepare) != 1 {
		t.Fatalf("expected 1 prepare, got %d", len(plan.Prepare))
	}
	if len(plan.Apply) != 2 {
		t.Fatalf("expected 2 apply (train+gguf), got %d", len(plan.Apply))
	}
}

func TestBuildPipelineMinimal(t *testing.T) {
	cfg := &TrainConfig{
		Method: MethodSFT,
		Model: ModelConfig{
			BaseModel: "Qwen/Qwen2.5-0.5B",
			HubID:     "dxrk/test-min",
		},
		Dataset: DatasetConfig{
			Name: "tatsu-lab/alpaca",
		},
		PushToHub:  false,
		ExportGGUF: false,
	}

	plan := BuildPipeline(cfg)
	if len(plan.Prepare) != 1 {
		t.Fatalf("expected 1 prepare, got %d", len(plan.Prepare))
	}
	if len(plan.Apply) != 1 {
		t.Fatalf("expected 1 apply (train only), got %d", len(plan.Apply))
	}
}

func TestTrainConfigJSON(t *testing.T) {
	cfg := TrainConfig{
		Method: MethodGRPO,
		Model: ModelConfig{
			BaseModel: "Qwen/Qwen2.5-0.5B",
			HubID:     "dxrk/test-grpo",
		},
		Dataset: DatasetConfig{
			Name: "openai/openai_hermes_rlprompt",
		},
		Hardware: HardwareA10GLarge,
		HP: Hyperparameters{
			LearningRate: 1e-5,
			BatchSize:    4,
			Epochs:       3,
			LoRARank:     16,
			LoRAAlpha:    32,
		},
		TimeoutMin: 120,
		PushToHub:  true,
	}

	if cfg.Method != MethodGRPO {
		t.Errorf("expected GRPO, got %s", cfg.Method)
	}
	if cfg.Hardware != HardwareA10GLarge {
		t.Errorf("expected a10g-large, got %s", cfg.Hardware)
	}
	if cfg.HP.LoRARank != 16 {
		t.Errorf("expected LoRA rank 16, got %d", cfg.HP.LoRARank)
	}
}
