// SPDX-License-Identifier: MIT
package ml

import (
	"fmt"
	"os/exec"
	"strings"
)

// TrainStep runs the TRL training job (SFT, DPO, GRPO, or reward modeling).
type TrainStep struct {
	Config *TrainConfig
}

func (s *TrainStep) ID() string { return "ml:train" }

func (s *TrainStep) Run() error {
	script := s.buildScript()

	cmd := exec.Command("uv", "run", "--with", "trl", "--with", "transformers", "--with", "datasets", "--with", "peft", "--with", "accelerate", "python", "-c", script)
	cmd.Env = append(cmd.Environ(), "HF_TOKEN="+s.Config.HFToken)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("training failed: %s\n%s", err, string(out))
	}

	fmt.Printf("  ✓ training completed\n")
	return nil
}

func (s *TrainStep) buildScript() string {
	cfg := s.Config
	var b strings.Builder

	b.WriteString("from trl import ")
	switch cfg.Method {
	case MethodSFT:
		b.WriteString("SFTTrainer, SFTConfig")
	case MethodDPO:
		b.WriteString("DPOTrainer, DPOConfig")
	case MethodGRPO:
		b.WriteString("GRPOTrainer, GRPOConfig")
	case MethodRewardModel:
		b.WriteString("RewardTrainer, RewardConfig")
	}
	b.WriteString("\n")
	b.WriteString("from transformers import AutoModelForCausalLM, AutoTokenizer\n")
	b.WriteString("from datasets import load_dataset\n")
	b.WriteString("import os\n\n")

	// Load dataset
	b.WriteString(fmt.Sprintf("dataset = load_dataset(%q", cfg.Dataset.Name))
	if cfg.Dataset.Subset != "" {
		b.WriteString(fmt.Sprintf(", %q", cfg.Dataset.Subset))
	}
	b.WriteString(")\n\n")

	// Load model
	b.WriteString(fmt.Sprintf("model = AutoModelForCausalLM.from_pretrained(%q)\n", cfg.Model.BaseModel))
	b.WriteString(fmt.Sprintf("tokenizer = AutoTokenizer.from_pretrained(%q)\n", cfg.Model.BaseModel))

	// Training args
	argType := strings.ToUpper(string(cfg.Method)) + "Config"
	b.WriteString(fmt.Sprintf("args = %s(\n", argType))
	b.WriteString(fmt.Sprintf("    output_dir=%q,\n", cfg.Model.HubID))
	if cfg.HP.LearningRate > 0 {
		b.WriteString(fmt.Sprintf("    learning_rate=%g,\n", cfg.HP.LearningRate))
	}
	if cfg.HP.BatchSize > 0 {
		b.WriteString(fmt.Sprintf("    per_device_train_batch_size=%d,\n", cfg.HP.BatchSize))
	}
	if cfg.HP.Epochs > 0 {
		b.WriteString(fmt.Sprintf("    num_train_epochs=%d,\n", cfg.HP.Epochs))
	}
	if cfg.HP.MaxSteps > 0 {
		b.WriteString(fmt.Sprintf("    max_steps=%d,\n", cfg.HP.MaxSteps))
	}
	if cfg.HP.WarmupRatio > 0 {
		b.WriteString(fmt.Sprintf("    warmup_ratio=%g,\n", cfg.HP.WarmupRatio))
	}
	if cfg.HP.GradientAccum > 0 {
		b.WriteString(fmt.Sprintf("    gradient_accumulation_steps=%d,\n", cfg.HP.GradientAccum))
	}
	if cfg.PushToHub {
		b.WriteString("    push_to_hub=True,\n")
		b.WriteString(fmt.Sprintf("    hub_model_id=%q,\n", cfg.Model.HubID))
	}
	b.WriteString(")\n\n")

	// Create trainer
	switch cfg.Method {
	case MethodSFT:
		b.WriteString("trainer = SFTTrainer(\n")
		b.WriteString("    model=model,\n")
		b.WriteString("    args=args,\n")
		b.WriteString("    train_dataset=dataset['train'],\n")
		b.WriteString("    processing_class=tokenizer,\n")
		b.WriteString(")\n")
	case MethodDPO:
		b.WriteString("trainer = DPOTrainer(\n")
		b.WriteString("    model=model,\n")
		b.WriteString("    args=args,\n")
		b.WriteString("    train_dataset=dataset['train'],\n")
		b.WriteString("    processing_class=tokenizer,\n")
		b.WriteString(")\n")
	case MethodGRPO:
		b.WriteString("trainer = GRPOTrainer(\n")
		b.WriteString("    model=model,\n")
		b.WriteString("    args=args,\n")
		b.WriteString("    train_dataset=dataset['train'],\n")
		b.WriteString("    processing_class=tokenizer,\n")
		b.WriteString(")\n")
	case MethodRewardModel:
		b.WriteString("trainer = RewardTrainer(\n")
		b.WriteString("    model=model,\n")
		b.WriteString("    args=args,\n")
		b.WriteString("    train_dataset=dataset['train'],\n")
		b.WriteString("    processing_class=tokenizer,\n")
		b.WriteString(")\n")
	}

	b.WriteString("\ntrainer.train()\n")
	if cfg.PushToHub {
		b.WriteString("trainer.push_to_hub()\n")
	}

	return b.String()
}
