// SPDX-License-Identifier: MIT
package ml

// TrainingMethod defines the TRL training strategy.
type TrainingMethod string

const (
	MethodSFT         TrainingMethod = "sft"
	MethodDPO         TrainingMethod = "dpo"
	MethodGRPO        TrainingMethod = "grpo"
	MethodRewardModel TrainingMethod = "reward"
)

// Hardware represents the compute target for training.
type Hardware string

const (
	HardwareT4Small   Hardware = "t4-small"
	HardwareT4Medium  Hardware = "t4-medium"
	HardwareA10GLarge Hardware = "a10g-large"
	HardwareA100Large Hardware = "a100-large"
)

// DatasetConfig describes the training dataset.
type DatasetConfig struct {
	Name   string `json:"name"`
	Subset string `json:"subset,omitempty"`
	Format string `json:"format,omitempty"` // "messages", "text", "prompt-completion", "chosen-rejected"
}

// ModelConfig describes the base model and output.
type ModelConfig struct {
	BaseModel string `json:"base_model"`
	Adapter   string `json:"adapter,omitempty"` // "lora", "qlora", "" for none
	HubID     string `json:"hub_id"`
	MaxSeqLen int    `json:"max_seq_len,omitempty"`
}

// Hyperparameters for training.
type Hyperparameters struct {
	LearningRate  float64 `json:"learning_rate,omitempty"`
	BatchSize     int     `json:"batch_size,omitempty"`
	Epochs        int     `json:"epochs,omitempty"`
	MaxSteps      int     `json:"max_steps,omitempty"`
	WarmupRatio   float64 `json:"warmup_ratio,omitempty"`
	WeightDecay   float64 `json:"weight_decay,omitempty"`
	LoRARank      int     `json:"loha_rank,omitempty"`
	LoRAAlpha     int     `json:"loha_alpha,omitempty"`
	LoRADropout   float64 `json:"loha_dropout,omitempty"`
	GradientAccum int     `json:"gradient_accumulation,omitempty"`
}

// TrainConfig is the top-level configuration for an ML training pipeline.
type TrainConfig struct {
	Method     TrainingMethod  `json:"method"`
	Model      ModelConfig     `json:"model"`
	Dataset    DatasetConfig   `json:"dataset"`
	Hardware   Hardware        `json:"hardware"`
	HP         Hyperparameters `json:"hyperparameters"`
	HFToken    string          `json:"-"`           // never serialized
	TimeoutMin int             `json:"timeout_min"` // job timeout in minutes
	PushToHub  bool            `json:"push_to_hub"`
	ExportGGUF bool            `json:"export_gguf,omitempty"`
}
