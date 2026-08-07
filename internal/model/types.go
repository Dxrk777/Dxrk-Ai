// SPDX-License-Identifier: MIT
package model

import "github.com/Dxrk777/Dxrk/internal/strconst"

type AgentID string

const (
	AgentClaudeCode    AgentID = "claude-code"
	AgentOpenCode      AgentID = "opencode"
	AgentKilocode      AgentID = "kilocode"
	AgentGeminiCLI     AgentID = "gemini-cli"
	AgentCursor        AgentID = "cursor"
	AgentVSCodeCopilot AgentID = "vscode-copilot"
	AgentCodex         AgentID = "codex"
	AgentAntigravity   AgentID = "antigravity"
	AgentWindsurf      AgentID = "windsurf"
	AgentKimi          AgentID = "kimi"
	AgentQwenCode      AgentID = "qwen-code"
	AgentKiroIDE       AgentID = "kiro-ide"
	AgentOpenClaw      AgentID = "openclaw"
	AgentPi            AgentID = "pi"
	AgentAider         AgentID = "aider"
	AgentCline         AgentID = "cline"
	AgentRooCode       AgentID = "roo-code"
	AgentContinue      AgentID = "continue"
	AgentJunie         AgentID = "junie"
	AgentAmazonQ       AgentID = "amazon-q"
	AgentOpenHands     AgentID = "openhands"
	AgentZedAI         AgentID = "zed-ai"
	AgentCopilot       AgentID = "github-copilot"
	AgentDevin         AgentID = "devin"
	AgentCody          AgentID = "cody"
	AgentTabnine       AgentID = "tabnine"
	AgentReplit        AgentID = "replit"
	AgentVoid          AgentID = "void"
	AgentHermes        AgentID = "hermes"
	AgentAmp           AgentID = "amp"
	AgentTrae          AgentID = "trae"
	AgentConductor     AgentID = "conductor"
	AgentRunCell       AgentID = "runcell"
	AgentLoopoperators AgentID = "looperators"
	AgentPearAI        AgentID = "pearai"
	AgentBolt          AgentID = "bolt"
	AgentLovable       AgentID = "lovable"
	AgentV0            AgentID = "v0"
	AgentBlackbox      AgentID = "blackbox"
	AgentQodo          AgentID = "qodo"
	AgentJetBrains     AgentID = "jetbrains"
	AgentZCode         AgentID = "zcode"
)

// SupportTier indicates how fully an agent supports the Dxrk AI ecosystem.
// All current agents receive the full SDD orchestrator, skill files, MCP config,
// and system prompt injection. The tier is kept as metadata for display purposes.
type SupportTier string

const (
	// TierFull — the agent receives all ecosystem features: SDD orchestrator,
	// skill files, MCP servers, system prompt, and sub-agent delegation.
	TierFull SupportTier = "full"
)

type ComponentID string

const (
	ComponentDxrkMemory        ComponentID = "dxrk-memory"
	ComponentSDD               ComponentID = "sdd"
	ComponentSkills            ComponentID = "skills"
	ComponentContext7          ComponentID = "context7"
	ComponentPersona           ComponentID = "persona"
	ComponentPermission        ComponentID = "permissions"
	ComponentDxrkGuardian      ComponentID = "dxrk-guardian"
	ComponentTheme             ComponentID = "theme"
	ComponentClaudeTheme       ComponentID = "claude-theme"
	ComponentOpenCodeDxrkLogo  ComponentID = "opencode-dxrk-logo"
	ComponentChecker           ComponentID = "checker"
	ComponentInternalMCPServer ComponentID = "internal-mcp-server"
)

type UninstallMode string

const (
	UninstallModePartial      UninstallMode = "partial"
	UninstallModeFull         UninstallMode = "full"
	UninstallModeFullRemove   UninstallMode = "full-remove"
	UninstallModeCleanInstall UninstallMode = "clean-install"
)

type DxrkMemoryUninstallScope string

const (
	DxrkMemoryUninstallScopeGlobal  DxrkMemoryUninstallScope = "global"
	DxrkMemoryUninstallScopeProject DxrkMemoryUninstallScope = strconst.StrProject
)

type SkillID string

const (
	SkillSDDInit         SkillID = "sdd-init"
	SkillSDDApply        SkillID = "sdd-apply"
	SkillSDDVerify       SkillID = "sdd-verify"
	SkillSDDExplore      SkillID = "sdd-explore"
	SkillSDDPropose      SkillID = "sdd-propose"
	SkillSDDSpec         SkillID = "sdd-spec"
	SkillSDDDesign       SkillID = "sdd-design"
	SkillSDDTasks        SkillID = "sdd-tasks"
	SkillSDDArchive      SkillID = "sdd-archive"
	SkillSDDOnboard      SkillID = "sdd-onboard"
	SkillGoTesting       SkillID = "go-testing"
	SkillCreator         SkillID = "skill-creator"
	SkillJudgmentDay     SkillID = "judgment-day"
	SkillBranchPR        SkillID = "branch-pr"
	SkillIssueCreation   SkillID = "issue-creation"
	SkillSkillRegistry   SkillID = "skill-registry"
	SkillChainedPR       SkillID = "chained-pr"
	SkillCognitiveDoc    SkillID = "cognitive-doc-design"
	SkillCommentWriter   SkillID = "comment-writer"
	SkillWorkUnitCommits SkillID = "work-unit-commits"
	SkillLLMCouncil      SkillID = "llm-council"

	// ── Programming Languages ──────────────────────────────────────
	SkillPythonPro             SkillID = "python-pro"
	SkillPythonPatterns        SkillID = "python-patterns"
	SkillAsyncPythonPatterns   SkillID = "async-python-patterns"
	SkillPythonFastapi         SkillID = "python-fastapi-development"
	SkillPythonPackaging       SkillID = "python-packaging"
	SkillPythonPerformance     SkillID = "python-performance-optimization"
	SkillPytestSkill           SkillID = "pytest-skill"
	SkillPydanticAI            SkillID = "pydantic-ai"
	SkillJavaScriptPro         SkillID = "javascript-pro"
	SkillJavaScriptMastery     SkillID = "javascript-mastery"
	SkillJavaScriptDesign      SkillID = "javascript-design-patterns"
	SkillNodejsPro             SkillID = "nodejs-pro"
	SkillTypeScriptPro         SkillID = "typescript-pro"
	SkillNextjsPro             SkillID = "nextjs-pro"
	SkillRustPro               SkillID = "rust-pro"
	SkillRustAsync             SkillID = "rust-async-patterns"
	SkillMemorySafetyPatterns  SkillID = "memory-safety-patterns"
	SkillGolangPro             SkillID = "golang-pro"
	SkillGoConcurrencyPatterns SkillID = "go-concurrency-patterns"
	SkillGoInDepth             SkillID = "go-in-depth"
	SkillGoPlaywright          SkillID = "go-playwright"
	SkillGoRodMaster           SkillID = "go-rod-master"
	SkillGrpcGolang            SkillID = "grpc-golang"
	SkillTemporalGolangPro     SkillID = "temporal-golang-pro"
	SkillJavaPro               SkillID = "java-pro"
	SkillSpringbootPro         SkillID = "springboot-pro"
	SkillJavaPerformanceTuning SkillID = "java-performance-tuning"
	SkillCppPro                SkillID = "cpp-pro"
	SkillCppLowLatency         SkillID = "cpp-low-latency"
	SkillSwiftPro              SkillID = "swift-pro"
	SkillSwiftConcurrency      SkillID = "swift-concurrency-expert"
	SkillSwiftuiExpert         SkillID = "swiftui-expert"
	SkillKotlinPro             SkillID = "kotlin-pro"
	SkillKotlinMultiplatform   SkillID = "kotlin-multiplatform"
	SkillAndroidCompose        SkillID = "android-jetpack-compose-expert"
	SkillRubyRailsPro          SkillID = "ruby-rails-pro"
	SkillPhpPro                SkillID = "php-pro"
	SkillPhpLaravelPro         SkillID = "php-laravel-pro"

	// ── Web / Frontend ─────────────────────────────────────────────
	SkillReactBestPractices SkillID = "react-best-practices"
	SkillReactPatterns      SkillID = "react-patterns"
	SkillAngularPro         SkillID = "angular-pro"
	SkillSveltePro          SkillID = "svelte-pro"
	SkillVuePro             SkillID = "vue-pro"
	SkillTailwindPro        SkillID = "tailwind-pro"
	SkillCssPro             SkillID = "css-pro"
	SkillHtmlPro            SkillID = "html-pro"
	SkillFrontendArch       SkillID = "frontend-architecture"
	SkillReactCompPerf      SkillID = "react-component-performance"

	// ── DevOps / Cloud ─────────────────────────────────────────────
	SkillDockerExpert          SkillID = "docker-expert"
	SkillKubernetesArchitect   SkillID = "kubernetes-architect"
	SkillTerraformPatterns     SkillID = "terraform-patterns"
	SkillAWSArchitect          SkillID = "aws-architect"
	SkillCloudArchitect        SkillID = "cloud-architect"
	SkillGitHubActionsAdvanced SkillID = "github-actions-advanced"
	SkillCICDPipelineBuilder   SkillID = "ci-cd-pipeline-builder"
	SkillGitopsWorkflow        SkillID = "gitops-workflow"
	SkillArgoCDPro             SkillID = "argocd-pro"
	SkillHelmChartBuilder      SkillID = "helm-chart-builder"
	SkillAWSLambdaPro          SkillID = "aws-lambda-pro"

	// ── AI / ML ────────────────────────────────────────────────────
	SkillPromptEngineering    SkillID = "prompt-engineering-patterns"
	SkillAgentDesigner        SkillID = "agent-designer"
	SkillMemorySystems        SkillID = "memory-systems"
	SkillLLMAppPatterns       SkillID = "llm-app-patterns"
	SkillLLMEvaluation        SkillID = "llm-evaluation"
	SkillRAGArchitect         SkillID = "rag-architect"
	SkillRAGEngineer          SkillID = "rag-engineer"
	SkillAIEngineeringToolkit SkillID = "ai-engineering-toolkit"
	SkillFineTuningPro        SkillID = "fine-tuning-pro"
	SkillLangChainPro         SkillID = "langchain-pro"
	SkillEmbeddingPro         SkillID = "embedding-pro"
	SkillVectorDBPro          SkillID = "vector-db-pro"
	SkillMLOpsPro             SkillID = "ml-ops-pro"
	SkillComputerVisionPro    SkillID = "computer-vision-pro"
	SkillNLPPro               SkillID = "nlp-pro"
	SkillHuggingFaceCLI       SkillID = "hugging-face-cli"
	SkillLangGraph            SkillID = "langgraph"

	// ── Data ───────────────────────────────────────────────────────
	SkillDataEngineer          SkillID = "data-engineer"
	SkillDataPipeline          SkillID = "data-engineering-data-pipeline"
	SkillPostgresBestPractices SkillID = "postgres-best-practices"
	SkillRedisPro              SkillID = "redis-pro"
	SkillMongoDBPro            SkillID = "mongodb-pro"
	SkillDataVisualization     SkillID = "data-visualization"
	SkillDBQuery               SkillID = "db-query"
	SkillMigration             SkillID = "migration"

	// ── Mobile ─────────────────────────────────────────────────────
	SkillReactNative   SkillID = "react-native"
	SkillAndroidDev    SkillID = "android-dev"
	SkillFlutterPro    SkillID = "flutter-pro"
	SkillIOSPro        SkillID = "ios-pro"
	SkillMobileTesting SkillID = "mobile-app-testing"

	// ── Security ───────────────────────────────────────────────────
	SkillSecuritySAST         SkillID = "security-scanning-security-sast"
	SkillSecurityHardening    SkillID = "security-scanning-security-hardening"
	SkillSecurityDependencies SkillID = "security-scanning-security-dependencies"
	SkillAPISecurity          SkillID = "api-security-best-practices"
	SkillContainerSecurity    SkillID = "container-security-hardening"
	SkillCloudSecurity        SkillID = "cloud-security"
	SkillPenetrationTesting   SkillID = "penetration-testing"

	// ── Testing ────────────────────────────────────────────────────
	SkillTDDGuide        SkillID = "tdd-guide"
	SkillE2ETesting      SkillID = "e2e-testing"
	SkillK6LoadTesting   SkillID = "k6-load-testing"
	SkillTestingPatterns SkillID = "testing-patterns"
	SkillTestAutomator   SkillID = "test-automator"
	SkillPlaywrightPro   SkillID = "playwright-pro"
	SkillCypressPro      SkillID = "cypress-pro"

	// ── Architecture ───────────────────────────────────────────────
	SkillSoftwareArchitecture   SkillID = "software-architecture"
	SkillMicroservicesPatterns  SkillID = "microservices-patterns"
	SkillEventSourcingArchitect SkillID = "event-sourcing-architect"
	SkillDDDPro                 SkillID = "ddd-pro"
	SkillSagaOrchestration      SkillID = "saga-orchestration"

	// ── CLI / Terminal ─────────────────────────────────────────────
	SkillBashPro       SkillID = "bash-pro"
	SkillBashScripting SkillID = "bash-scripting"
	SkillPosixShellPro SkillID = "posix-shell-pro"
	SkillAiNativeCLI   SkillID = "ai-native-cli"
	SkillJq            SkillID = "jq"

	// ── Documentation ──────────────────────────────────────────────
	SkillAPIDocs       SkillID = "api-docs"
	SkillDocGeneration SkillID = "documentation-generation"
	SkillChangelogPro  SkillID = "changelog-pro"
	SkillReadmePro     SkillID = "readme-pro"

	// ── Images / Media ─────────────────────────────────────────────
	SkillImageGeneration SkillID = "image-generation"
	SkillVideoEditing    SkillID = "video-editing"
	SkillAudioProcessing SkillID = "audio-processing"
	Skill3DModeling      SkillID = "3d-modeling"
	SkillAlgorithmicArt  SkillID = "algorithmic-art"

	// ── PDF / Documents ────────────────────────────────────────────
	SkillPDFGeneration SkillID = "pdf-generation"
	SkillWordDocx      SkillID = "word-docx"
	SkillExcelXlsx     SkillID = "excel-xlsx"
	SkillPPTXDeck      SkillID = "pptx-deck-creation"

	// ── Business ───────────────────────────────────────────────────
	SkillProductManagement SkillID = "product-management"
	SkillAgileScrum        SkillID = "agile-scrum"
	SkillOKRTracking       SkillID = "okr-tracking"

	// ── Writing ────────────────────────────────────────────────────
	SkillTechnicalWriting SkillID = "technical-writing"
	SkillCopywriting      SkillID = "copywriting"
	SkillSEOWriting       SkillID = "seo-writing"
	SkillBlogWriting      SkillID = "blog-writing"

	// ── Code Quality ───────────────────────────────────────────────
	SkillCodeReviewChecklist   SkillID = "code-review-checklist"
	SkillRefactoringPatterns   SkillID = "refactoring-patterns"
	SkillErrorHandlingPatterns SkillID = "error-handling-patterns"
	SkillSystematicDebugging   SkillID = "systematic-debugging"
	SkillCodeSimplification    SkillID = "code-simplification"

	// ── Observability ──────────────────────────────────────────────
	SkillObservability     SkillID = "observability-and-instrumentation"
	SkillIncidentResponder SkillID = "incident-responder"
	SkillPostmortem        SkillID = "postmortem"
	SkillChaosEngineering  SkillID = "chaos-engineering"

	// ── Workflow extras ────────────────────────────────────────────
	SkillAccessiblity    SkillID = "accessibility"
	SkillArchDecision    SkillID = "arch-decision"
	SkillCICD            SkillID = "ci-cd"
	SkillCodeReview      SkillID = "code-review"
	SkillCommitMessage   SkillID = "commit-message"
	SkillDebugging       SkillID = "debugging"
	SkillDependency      SkillID = "dependency"
	SkillDockerMgmt      SkillID = "docker-mgmt"
	SkillEnvSetup        SkillID = "env-setup"
	SkillErrorHandling   SkillID = "error-handling"
	SkillGitRelease      SkillID = "git-release"
	SkillLoggingPatterns SkillID = "logging-patterns"
	SkillPerformance     SkillID = "performance"
	SkillPRDescription   SkillID = "pr-description"
	SkillRefactoringPR   SkillID = "refactoring-pr"
	SkillSecurityAudit   SkillID = "security-audit"
	SkillTestWriter      SkillID = "test-writer"

	// ── Dxrk-specific ─────────────────────────────────────────────
	SkillDxrkAPIContent      SkillID = "dxrk-api-content"
	SkillDxrkBatch           SkillID = "dxrk-batch"
	SkillDxrkClaudeAPI       SkillID = "dxrk-claude-api"
	SkillDxrkClaudeChrome    SkillID = "dxrk-claude-chrome"
	SkillDxrkDebug           SkillID = "dxrk-debug"
	SkillDxrkDiscordAgent    SkillID = "dxrk-discord-agent"
	SkillDxrkDream           SkillID = "dxrk-dream"
	SkillDxrkDuplicateDetect SkillID = "dxrk-duplicate-detection"
	SkillDxrkGHSA            SkillID = "dxrk-ghsa-maintainer"
	SkillDxrkGitcrawl        SkillID = "dxrk-gitcrawl"
	SkillDxrkKeybindings     SkillID = "dxrk-keybindings"
	SkillDxrkLoop            SkillID = "dxrk-loop"
	SkillDxrkLoremIpsum      SkillID = "dxrk-lorem-ipsum"
	SkillDxrkParallelsE2E    SkillID = "dxrk-parallels-e2e"
	SkillDxrkParallelsSmoke  SkillID = "dxrk-parallels-smoke"
	SkillDxrkPreRelease      SkillID = "dxrk-pre-release-testing"
	SkillDxrkPRMaintainer    SkillID = "dxrk-pr-maintainer"
	SkillDxrkQATesting       SkillID = "dxrk-qa-testing"
	SkillDxrkRelease         SkillID = "dxrk-release-maintainer"
	SkillDxrkRemember        SkillID = "dxrk-remember"
	SkillDxrkScheduleAgents  SkillID = "dxrk-schedule-agents"
	SkillDxrkSecretScan      SkillID = "dxrk-secret-scanning" //nolint:gosec // not a real secret
	SkillDxrkSecurityTriage  SkillID = "dxrk-security-triage"
	SkillDxrkSimplify        SkillID = "dxrk-simplify"
	SkillDxrkSkillGenerator  SkillID = "dxrk-skill-generator"
	SkillDxrkSkillify        SkillID = "dxrk-skillify"
	SkillDxrkStuck           SkillID = "dxrk-stuck"
	SkillDxrkTestbox         SkillID = "dxrk-testbox"
	SkillDxrkTesting         SkillID = "dxrk-testing"
	SkillDxrkTestMemory      SkillID = "dxrk-test-memory"
	SkillDxrkTestOptimize    SkillID = "dxrk-test-optimize"
	SkillDxrkTestPerformance SkillID = "dxrk-test-performance"
	SkillDxrkUpdateConfig    SkillID = "dxrk-update-config"
	SkillDxrkVerify          SkillID = "dxrk-verify"

	// ── Additional Languages ──────────────────────────────────────
	SkillTypeScriptExpert    SkillID = "typescript-expert"
	SkillNodejsBackend       SkillID = "nodejs-backend-patterns"
	SkillNodejsBestPractices SkillID = "nodejs-best-practices"
	SkillTrpcFullstack       SkillID = "trpc-fullstack"
	SkillDrizzleORM          SkillID = "drizzle-orm-expert"
	SkillPrismaExpert        SkillID = "prisma-expert"
	SkillRubyPro             SkillID = "ruby-pro"
	SkillPythonTesting       SkillID = "python-testing-patterns"

	// ── Additional Web / Frontend ─────────────────────────────────
	SkillAngular              SkillID = "angular"
	SkillAngularBestPractices SkillID = "angular-best-practices"
	SkillSvelteKit            SkillID = "sveltekit"
	SkillTailwindPatterns     SkillID = "tailwind-patterns"
	SkillReactStateMgmt       SkillID = "react-state-management"
	SkillReactComponentPerf2  SkillID = "react-component-performance-2"

	// ── Additional DevOps / Cloud ─────────────────────────────────
	SkillAWSServerlessEDA   SkillID = "aws-serverless-eda"
	SkillAWSPenetration     SkillID = "aws-penetration-testing"
	SkillAzureCloud         SkillID = "azure-cloud-architect"
	SkillDockerDevelopment  SkillID = "docker-development"
	SkillDeploymentPipeline SkillID = "deployment-pipeline-design"
	SkillCICDAutomation     SkillID = "ci-cd-and-automation"
	SkillHelmChartBuilder2  SkillID = "helm-chart-builder-2"
	SkillKubernetesDeploy   SkillID = "kubernetes-deployment"
	SkillKubernetesOperator SkillID = "kubernetes-operator"
	SkillSparkOptimization  SkillID = "spark-optimization"
	SkillSnowflake          SkillID = "snowflake-development"

	// ── Additional AI / ML ────────────────────────────────────────
	SkillAIAgentsArchitect   SkillID = "ai-agents-architect"
	SkillAgentProtocol       SkillID = "agent-protocol"
	SkillAgentMemorySystems  SkillID = "agent-memory-systems"
	SkillAISecurity          SkillID = "ai-security"
	SkillRAGImplementation   SkillID = "rag-implementation"
	SkillPromptEngineer      SkillID = "prompt-engineer"
	SkillPromptEngineering2  SkillID = "prompt-engineering"
	SkillHuggingFaceTrainer  SkillID = "hugging-face-model-trainer"
	SkillEmbeddingStrategies SkillID = "embedding-strategies"
	SkillLangChainArchitect  SkillID = "langchain-architecture"

	// ── Additional Data ───────────────────────────────────────────
	SkillDatabaseArchitect    SkillID = "database-architect"
	SkillDatabaseOptimizer    SkillID = "database-optimizer"
	SkillSQLDatabase          SkillID = "sql-database-assistant"
	SkillPostgreSQL           SkillID = "postgresql"
	SkillPostgresOptimization SkillID = "postgresql-optimization"
	SkillSupabase             SkillID = "supabase"

	// ── Additional Security ───────────────────────────────────────
	SkillSecurityAndHardening SkillID = "security-and-hardening"
	SkillSecurityAuditor      SkillID = "security-auditor"
	SkillSecurityPenTesting   SkillID = "security-pen-testing"
	SkillSecurityGuidance     SkillID = "security-guidance"
	SkillSecurityBluebook     SkillID = "security-bluebook-builder"
	SkillSecurityReqExtract   SkillID = "security-requirement-extraction"
	SkillSecretsManagement    SkillID = "secrets-management"
	SkillAPISecurityTesting   SkillID = "api-security-testing"

	// ── Additional Testing ────────────────────────────────────────
	SkillCypressSkill     SkillID = "cypress-skill"
	SkillPlaywrightSkill  SkillID = "playwright-skill"
	SkillPlaywrightJava   SkillID = "playwright-java"
	SkillUnitTestGenerate SkillID = "unit-testing-test-generate"
	SkillTestingQA        SkillID = "testing-qa"
	SkillTDDDrive         SkillID = "test-driven-development"
	SkillAPITestSuite     SkillID = "api-test-suite-builder"
	SkillAPITestMock      SkillID = "api-testing-observability-api-mock"

	// ── Additional Architecture ───────────────────────────────────
	SkillArchitecturePatterns SkillID = "architecture-patterns"
	SkillBackendArchitect     SkillID = "backend-architect"
	SkillDomainDrivenDesign   SkillID = "domain-driven-design"

	// ── Additional Documentation ──────────────────────────────────
	SkillDocumentation      SkillID = "documentation"
	SkillDocumentationADRs  SkillID = "documentation-and-adrs"
	SkillOpenAPISpec        SkillID = "openapi-spec-generator"
	SkillChangelogGenerator SkillID = "changelog-generator"
	SkillReadme             SkillID = "readme"
	SkillDocx               SkillID = "docx"
	SkillXLSX               SkillID = "xlsx"
	SkillPPTX               SkillID = "pptx"
	SkillPDF                SkillID = "pdf"
	SkillPDFOfficial        SkillID = "pdf-official"

	// ── Additional Media ──────────────────────────────────────────
	Skill3DWebExperience     SkillID = "3d-web-experience"
	SkillThreeJSFundamentals SkillID = "threejs-fundamentals"
	SkillThreeJSAnimation    SkillID = "threejs-animation"
	SkillAIStudioImage       SkillID = "ai-studio-image"
	SkillAudioTranscriber    SkillID = "audio-transcriber"
	SkillDemoVideo           SkillID = "demo-video"

	// ── Additional Business / Writing ─────────────────────────────
	SkillAgileProductOwner  SkillID = "agile-product-owner"
	SkillProductManagerTool SkillID = "product-manager-toolkit"
	SkillProductStrategist  SkillID = "product-strategist"
	SkillScrumMaster        SkillID = "scrum-master"
	SkillContentHumanizer   SkillID = "content-humanizer"
	SkillDevRelContent      SkillID = "devrel-content"
	SkillScientificWriting  SkillID = "scientific-writing"
	SkillPostmortemWriting  SkillID = "postmortem-writing"
	SkillCopywritingPro     SkillID = "copywriting-pro"

	// ── Additional Code Quality ───────────────────────────────────
	SkillBrooksLint       SkillID = "brooks-lint"
	SkillCodeReviewer     SkillID = "code-reviewer"
	SkillCaveman          SkillID = "caveman"
	SkillUnslop           SkillID = "unslop"
	SkillUnslopCommit     SkillID = "unslop-commit"
	SkillUnslopFile       SkillID = "unslop-file"
	SkillUnslopReview     SkillID = "unslop-review"
	SkillGrillMe          SkillID = "grill-me"
	SkillGrilling         SkillID = "grilling"
	SkillGrillWithDocs    SkillID = "grill-with-docs"
	SkillHandoff          SkillID = "handoff"
	SkillLast30Days       SkillID = "last30days"
	SkillCommit           SkillID = "commit"
	SkillPRWriter         SkillID = "pr-writer"
	SkillSkillOptimizer   SkillID = "skill-optimizer"
	SkillSuperpowersLab   SkillID = "superpowers-lab"
	SkillUsingSuperpowers SkillID = "using-superpowers"

	// ── Additional Observability / Debugging ──────────────────────
	SkillDatadogAutomation    SkillID = "datadog-automation"
	SkillDebuggingCode        SkillID = "debugging-code"
	SkillDebuggingStrategies  SkillID = "debugging-strategies"
	SkillDebuggingToolkit     SkillID = "debugging-toolkit"
	SkillDebuggingRecovery    SkillID = "debugging-and-error-recovery"
	SkillPerformanceEngineer  SkillID = "performance-engineer"
	SkillPerformanceOptim     SkillID = "performance-optimization"
	SkillPerformanceOptimizer SkillID = "performance-optimizer"
	SkillPerformanceProfiler  SkillID = "performance-profiler"
	SkillPerformanceProfiling SkillID = "performance-profiling"

	// ── MCP / n8n / Notion / Figma ───────────────────────────────
	SkillMCPBuilder             SkillID = "mcp-builder"
	SkillMCPBuilderMS           SkillID = "mcp-builder-ms"
	SkillMCPToolDeveloper       SkillID = "mcp-tool-developer"
	SkillN8nAgents              SkillID = "n8n-agents"
	SkillN8nBinaryData          SkillID = "n8n-binary-and-data"
	SkillN8nCodeJS              SkillID = "n8n-code-javascript"
	SkillN8nCodePython          SkillID = "n8n-code-python"
	SkillN8nCodeTool            SkillID = "n8n-code-tool"
	SkillN8nErrorHandling       SkillID = "n8n-error-handling"
	SkillNotionAutomation       SkillID = "notion-automation"
	SkillNotionTemplate         SkillID = "notion-template-business"
	SkillFigmaAutomation        SkillID = "figma-automation"
	SkillGitHubActionsDebug     SkillID = "github-actions-debugger"
	SkillGitHubActionsTemplates SkillID = "github-actions-templates"
	SkillGrafanaDashboards      SkillID = "grafana-dashboards"
	SkillAgentMemoryMCP         SkillID = "agent-memory-mcp"
	SkillHeliumMCP              SkillID = "helium-mcp"
	SkillHFMCP                  SkillID = "hf-mcp"
	SkillMercuryMCP             SkillID = "mercury-mcp"

	// ── Environment / DX ──────────────────────────────────────────
	SkillEnvGuide          SkillID = "environment-setup-guide"
	SkillEnvSecretsManager SkillID = "env-secrets-manager" //nolint:gosec // not a real secret
	SkillCodexProfiles     SkillID = "codex-profiles"
	SkillCodexReview       SkillID = "codex-review"
	SkillCodexSubagent     SkillID = "codex-subagent"
	SkillCodeReviewOpt     SkillID = "code-review-optimization"
)

type PersonaID string

const (
	PersonaDxrk    PersonaID = "dxrk"
	PersonaNeutral PersonaID = "neutral"
	PersonaCustom  PersonaID = "custom"
)

// SystemPromptStrategy defines how an agent's system prompt file is managed.
type SystemPromptStrategy int

const (
	// StrategyMarkdownSections uses <!-- dxrk:ID --> markers to inject sections
	// into an existing file without clobbering user content (Claude Code CLAUDE.md).
	StrategyMarkdownSections SystemPromptStrategy = iota
	// StrategyFileReplace replaces the entire system prompt file (OpenCode AGENTS.md).
	StrategyFileReplace
	// StrategyAppendToFile appends content to an existing system prompt file.
	StrategyAppendToFile
	// StrategyInstructionsFile writes a dedicated instructions file (e.g. .instructions.md).
	StrategyInstructionsFile
	// StrategyJinjaModules writes separate module files that are included into a
	// thin Jinja2 template (e.g. Kimi's KIMI.md).
	StrategyJinjaModules
	// StrategySteeringFile writes a Kiro steering file with inclusion: always frontmatter.
	StrategySteeringFile
)

// MCPStrategy defines how MCP server configs are written for an agent.
type MCPStrategy int

const (
	// StrategySeparateMCPFiles writes one JSON file per server in a dedicated directory
	// (e.g., ~/.claude/mcp/context7.json).
	StrategySeparateMCPFiles MCPStrategy = iota
	// StrategyMergeIntoSettings merges mcpServers into a settings.json file
	// (e.g., OpenCode, Gemini CLI).
	StrategyMergeIntoSettings
	// StrategyMCPConfigFile writes to a dedicated mcp.json config file (e.g., Cursor ~/.cursor/mcp.json).
	StrategyMCPConfigFile
	// StrategyTOMLFile writes MCP config to a TOML file (e.g., Codex ~/.codex/config.toml).
	StrategyTOMLFile
)

type PresetID string

const (
	PresetFullDxrk      PresetID = "full-dxrk"
	PresetEcosystemOnly PresetID = "ecosystem-only"
	PresetMinimal       PresetID = "minimal"
	PresetCustom        PresetID = "custom"
)

type SDDModeID string

const (
	SDDModeSingle SDDModeID = "single"
	SDDModeMulti  SDDModeID = "multi"
)

// SDDProfileStrategyID defines how sync handles OpenCode SDD profiles.
type SDDProfileStrategyID string

const (
	// SDDProfileStrategyGeneratedMulti is the default/backward-compatible mode:
	// named profiles coexist in opencode.json as suffixed agents and are detected
	// from sdd-orchestrator-{name} keys during regular sync.
	SDDProfileStrategyGeneratedMulti SDDProfileStrategyID = "generated-multi"
	// SDDProfileStrategyExternalSingleActive supports external profile managers
	// that keep profile state outside opencode.json and activate one runtime
	// profile without requiring a restart.
	SDDProfileStrategyExternalSingleActive SDDProfileStrategyID = "external-single-active"
)

type OpenCodeCommunityPluginID string

const (
	OpenCodePluginSubAgentStatusline  OpenCodeCommunityPluginID = "sub-agent-statusline"
	OpenCodePluginSDDDxrkMemoryManage OpenCodeCommunityPluginID = "sdd-dxrk-memory-plugin"
	OpenCodePluginDxrkLogo            OpenCodeCommunityPluginID = "dxrk-logo"
)

// Profile represents a named SDD orchestrator configuration with model assignments.
// The default profile (Name="" or Name="default") maps to the base sdd-orchestrator.
// Named profiles generate sdd-orchestrator-{Name} + suffixed sub-agents.
type Profile struct {
	Name              string                     // e.g. "cheap", "premium"; empty = default
	OrchestratorModel ModelAssignment            // orchestrator model
	PhaseAssignments  map[string]ModelAssignment // key = phase name (e.g. "sdd-apply")
}
