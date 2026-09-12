package datasyncui

import (
	"context"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type RuleDatabaseScope struct {
	RuleID string `json:"rule_id,omitempty"`
	Side   string `json:"side,omitempty"`
}

type RuleQueryRequest struct {
	RuleDatabaseScope
	dbgovernance.QueryRequest
}
type RuleMutationRequest struct {
	RuleDatabaseScope
	dbgovernance.MutationRequest
}

func (s *MCPService) ruleDatabase(scope RuleDatabaseScope, table string) (string, error) {
	if scope.RuleID == "" {
		if scope.Side != "" {
			return "", fmt.Errorf("side requires rule_id")
		}
		return s.Config.MySQL.Database, nil
	}
	if scope.Side != "source" && scope.Side != "target" {
		return "", fmt.Errorf("side must be source or target")
	}
	set := s.app.ruleSet
	if s.RulesPath != "" {
		loaded, err := rules.LoadFile(s.RulesPath)
		if err != nil {
			return "", err
		}
		set = loaded
	}
	rule, err := findRuleByID(set, scope.RuleID)
	if err != nil {
		return "", err
	}
	database, expectedTable := rule.DatabaseName, rule.TableName
	if scope.Side == "target" {
		if rule.TargetDatabaseName != "" {
			database = rule.TargetDatabaseName
		}
		if s.Config.Mode == appconfig.ModeEdge {
			database = rule.DownlinkTargetDatabase(s.Config.MySQL.Database)
		}
		if rule.TargetTableName != "" {
			expectedTable = rule.TargetTableName
		}
	}
	if table != expectedTable {
		return "", fmt.Errorf("table must match the selected rule side")
	}
	if err := mapper.ValidateIdentifier(database); err != nil {
		return "", err
	}
	if err := mapper.ValidateIdentifier(table); err != nil {
		return "", err
	}
	return database, nil
}

func (s *MCPService) mysqlGovernanceQuery(req RuleQueryRequest) (dbgovernance.QueryResult, error) {
	database, err := s.ruleDatabase(req.RuleDatabaseScope, req.Table)
	if err != nil {
		return dbgovernance.QueryResult{}, err
	}
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return dbgovernance.QueryResult{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return dbgovernance.New(db, database).Query(ctx, req.QueryRequest)
}

func (s *MCPService) mysqlGovernanceMutationPlan(req RuleMutationRequest) (dbgovernance.MutationPlan, error) {
	database, err := s.ruleDatabase(req.RuleDatabaseScope, req.Table)
	if err != nil {
		return dbgovernance.MutationPlan{}, err
	}
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return dbgovernance.MutationPlan{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return dbgovernance.New(db, database).PlanMutation(ctx, req.MutationRequest)
}

func (s *MCPService) mysqlGovernanceMutationApply(req RuleMutationRequest) (dbgovernance.MutationResult, error) {
	database, err := s.ruleDatabase(req.RuleDatabaseScope, req.Table)
	if err != nil {
		return dbgovernance.MutationResult{}, err
	}
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return dbgovernance.MutationResult{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := dbgovernance.New(db, database).ApplyMutation(ctx, req.MutationRequest)
	if err != nil {
		return dbgovernance.MutationResult{Operation: req.Operation, Database: database, Table: req.Table, Status: "rejected"}, fmt.Errorf("governed mutation rejected: %w", err)
	}
	return result, nil
}

func (s *MCPService) mysqlGovernanceSchemaPlan(req dbgovernance.SchemaChangeRequest) (dbgovernance.SchemaChangePlan, error) {
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return dbgovernance.SchemaChangePlan{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return dbgovernance.New(db, s.Config.MySQL.Database).PlanSchemaChange(ctx, req)
}

func (s *MCPService) mysqlGovernanceSchemaApply(req dbgovernance.SchemaChangeApplyRequest) (dbgovernance.SchemaChangeResult, error) {
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return dbgovernance.SchemaChangeResult{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := dbgovernance.New(db, s.Config.MySQL.Database).ApplySchemaChange(ctx, req)
	if err != nil {
		return dbgovernance.SchemaChangeResult{Operation: req.Change.Operation, Database: s.Config.MySQL.Database, Table: req.Change.Table, Column: req.Change.Column.Name, Status: "rejected", PlanID: req.PlanID}, fmt.Errorf("governed schema change rejected: %w", err)
	}
	return result, nil
}
