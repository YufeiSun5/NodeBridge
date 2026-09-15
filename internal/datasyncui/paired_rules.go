package datasyncui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func (a *App) preparePairedRuleSave(ctx context.Context, set *rules.RuleSet) error {
	pairs, err := rulecheck.ReadPairManifest(a.effectiveRulesPath() + ".pairs.json")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.config == nil {
		return errConfigMissing
	}
	if _, err := rulecheck.CompileEndpoint(a.config.Node.ID, a.config.Mode, *set, pairs); err != nil {
		return err
	}
	needsProof := false
	for _, rule := range set.Rules {
		needsProof = needsProof || rule.Enable && rule.Direction == rules.DirectionBidirectional
	}
	if !needsProof {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := mysqlconn.Open(a.config.MySQL)
	if err != nil {
		return err
	}
	defer db.Close()
	proofs, err := alignment.LoadActiveCutovers(ctx, db)
	if err != nil {
		return err
	}
	if err := alignment.VerifyObservedPairs(pairs, proofs); err != nil {
		return err
	}
	return bindReadyRules(set, proofs, a.config.Node.ID)
}

func bindReadyRules(set *rules.RuleSet, proofs []alignment.CutoverProof, node string) error {
	for i, rule := range set.Rules {
		if !rule.Enable || rule.Direction != rules.DirectionBidirectional {
			continue
		}
		found := false
		for _, proof := range proofs {
			if _, err := proof.Local(node); err == nil && proof.MatchesRule(rule) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("alignment_active_proof_required: %s", rule.ID)
		}
		set.Rules[i] = rule.BindPairedRuntime()
	}
	return nil
}
