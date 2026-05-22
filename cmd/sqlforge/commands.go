package main

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"os"

	"github.com/drover-org/drover-sqlforge/internal/ai"
	"github.com/drover-org/drover-sqlforge/internal/config"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/drover-org/drover-sqlforge/internal/plan"
	"github.com/drover-org/drover-sqlforge/internal/project"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

var dims []string
var envBaseFlag string

func init() {
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(aiCmd)

	queryCmd.Flags().StringSliceVar(&dims, "dimensions", []string{}, "Dimensions to group by (e.g., metric_date,country)")
	rootCmd.AddCommand(queryCmd)

	envCmd.AddCommand(envCreateCmd)
	envCreateCmd.Flags().StringVar(&envBaseFlag, "base-env", "", "Parent environment (default: default_environment from sqlforge.yml or prod)")
}

func loadRuntime(envName string) (*project.Runtime, error) {
	return project.LoadRuntime(".", envName)
}

func runPipeline(envName string) (*plan.ExecutionPlan, *project.Runtime, error) {
	rt, err := loadRuntime(envName)
	if err != nil {
		return nil, nil, err
	}
	execPlan, err := rt.ExecutionPlan()
	if err != nil {
		rt.Close()
		return nil, nil, fmt.Errorf("error generating plan: %w", err)
	}
	return execPlan, rt, nil
}

var planCmd = &cobra.Command{
	Use:   "plan [environment]",
	Short: "Show what will change",
	Run: func(cmd *cobra.Command, args []string) {
		envName := "dev"
		if len(args) > 0 {
			envName = args[0]
		}

		fmt.Printf("Generating plan for environment: %s\n", envName)
		execPlan, rt, err := runPipeline(envName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer rt.Close()

		fmt.Printf("\nExecution Plan:\n")
		fmt.Printf("  Changed Models: %d\n", len(execPlan.ChangedModels))
		for _, m := range execPlan.ChangedModels {
			fmt.Printf("    - %s\n", m.Name)
		}
		fmt.Printf("  Impacted Models: %d\n", len(execPlan.Impacted))
		for _, m := range execPlan.Impacted {
			fmt.Printf("    - %s\n", m.Name)
		}
		fmt.Printf("  Unchanged Models: %d\n", len(execPlan.Unchanged))

		if len(execPlan.ChangedModels) == 0 && len(execPlan.Impacted) == 0 {
			fmt.Println("\nNothing to do. Environment is up to date.")
		}
	},
}

var applyCmd = &cobra.Command{
	Use:   "apply [environment]",
	Short: "Execute the plan safely",
	Run: func(cmd *cobra.Command, args []string) {
		envName := "dev"
		if len(args) > 0 {
			envName = args[0]
		}

		fmt.Printf("Generating plan for environment: %s\n", envName)
		execPlan, rt, err := runPipeline(envName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer rt.Close()

		if len(execPlan.ChangedModels) == 0 && len(execPlan.Impacted) == 0 {
			fmt.Println("\nNothing to do. Environment is up to date.")
			return
		}

		if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			if err := plan.ApplyPlan(context.Background(), execPlan, rt.StateMgr, rt.VMgr, nil); err != nil {
				fmt.Printf("Error applying plan: %v\n", err)
				os.Exit(1)
			}
			return
		}

		eventChan := make(chan plan.ApplyEvent)
		total := len(execPlan.ChangedModels) + len(execPlan.Impacted)

		var applyErr error
		go func() {
			applyErr = plan.ApplyPlan(context.Background(), execPlan, rt.StateMgr, rt.VMgr, eventChan)
			close(eventChan)
		}()

		p := tea.NewProgram(initialModel(eventChan, total))
		if _, err := p.Run(); err != nil {
			fmt.Printf("TUI Error: %v\n", err)
			os.Exit(1)
		}

		if applyErr != nil {
			os.Exit(1)
		}
	},
}

var queryCmd = &cobra.Command{
	Use:   "query [metric] [environment]",
	Short: "Query the semantic layer to generate dialect-agnostic SQL",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			fmt.Println("Error: usage query [metric] [environment]")
			return
		}
		metricName := args[0]
		envName := args[1]

		// 1. Setup state to get environment schema
		stateMgr, err := state.NewManager(".")
		if err != nil {
			fmt.Printf("Error setting up state manager: %v\n", err)
			return
		}

		env, err := stateMgr.GetOrCreateEnv(envName, "prod")
		if err != nil {
			fmt.Printf("Error getting env: %v\n", err)
			return
		}

		// 2. Load metrics
		graph, err := semantic.LoadMetrics(".")
		if err != nil {
			fmt.Printf("Error loading metrics: %v\n", err)
			return
		}

		metric := graph.FindMetric(metricName)
		if metric == nil {
			fmt.Printf("Error: metric '%s' not found.\n", metricName)
			return
		}

		// 3. Compile ANSI SQL
		compiler := semantic.NewCompiler(env.Schema)
		sql, err := compiler.Compile(metric, dims)
		if err != nil {
			fmt.Printf("Error compiling query: %v\n", err)
			return
		}

		fmt.Printf("\n--- Compiled ANSI SQL for '%s' ---\n\n", metricName)
		fmt.Println(sql)
		fmt.Println("\n----------------------------------")
	},
}

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
}

var envCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new environment",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Error: name is required")
			return
		}

		baseEnv := envBaseFlag
		if baseEnv == "" {
			if cfg, err := config.LoadConfig("."); err == nil && cfg.DefaultEnvironment != "" {
				baseEnv = cfg.DefaultEnvironment
			} else {
				baseEnv = "prod"
			}
		}

		stateMgr, err := state.NewManager(".")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(".")
		var runner virtual.Runner
		if err != nil {
			runner, _ = virtual.NewRunner("clickhouse", "")
		} else {
			runner, err = virtual.NewRunner(cfg.Virtual.Dialect, cfg.Virtual.Connection)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		}

		vMgr := virtual.NewManager(runner, stateMgr)
		if err := vMgr.CreateVirtualEnv(context.Background(), args[0], baseEnv); err != nil {
			fmt.Printf("Error creating environment: %v\n", err)
			os.Exit(1)
		}

		env, err := stateMgr.GetOrCreateEnv(args[0], baseEnv)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created environment %s (warehouse schema %s, base %s)\n", env.Name, env.Schema, env.BaseEnv)
	},
}

var runCmd = &cobra.Command{
	Use:   "run [environment]",
	Short: "Run models (with plan under the hood)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running models...")
	},
}

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse all models + show AST info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Parsing models...")
	},
}

var aiCmd = &cobra.Command{
	Use:   "ai explain [model]",
	Short: "Explain what a model does",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 || args[0] != "explain" {
			fmt.Println("Error: usage ai explain [model]")
			return
		}
		modelName := args[1]
		fmt.Printf("Explaining model %s...\n", modelName)

		// 1. Load config
		cfg, err := config.LoadConfig(".")
		if err != nil {
			fmt.Printf("Warning: could not load config (%v), using defaults.\n", err)
			cfg = &config.Config{
				AI: config.AIConfig{
					Provider: "ollama",
					Endpoint: "http://localhost:11434",
					Model:    "llama3.2",
				},
			}
		}

		// 2. Setup parser & load model
		ctx := context.Background()
		p, err := parser.NewParser(ctx)
		if err != nil {
			fmt.Printf("Error setting up parser: %v\n", err)
			return
		}
		defer p.Close()

		assets, err := model.LoadModels("models", p)
		if err != nil {
			fmt.Printf("Error loading models: %v\n", err)
			return
		}

		var targetModel *model.Asset
		for _, a := range assets {
			if a.Name == modelName {
				targetModel = a
				break
			}
		}

		if targetModel == nil {
			fmt.Printf("Error: model '%s' not found.\n", modelName)
			return
		}

		// 3. Call AI
		client := ai.NewClient(cfg.AI.Endpoint, cfg.AI.Model)

		fmt.Printf("Sending request to %s (Model: %s)...\n", cfg.AI.Endpoint, cfg.AI.Model)
		explanation, err := client.Explain(targetModel.SQL)
		if err != nil {
			fmt.Printf("Error calling AI: %v\n", err)
			return
		}

		fmt.Printf("\n--- Explanation for %s ---\n\n%s\n", targetModel.Name, explanation)
	},
}
