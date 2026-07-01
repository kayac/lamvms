package lamvms

import (
	"context"
	"fmt"
	"sync"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/go-jsonnet"
	jsonnetAst "github.com/google/go-jsonnet/ast"
)

type callerIdentity struct {
	cfg     aws.Config
	ctx     context.Context
	once    sync.Once
	account string
	arn     string
	userID  string
	err     error
}

func newCallerIdentity(cfg aws.Config, ctx context.Context) *callerIdentity {
	return &callerIdentity{cfg: cfg, ctx: ctx}
}

func (c *callerIdentity) resolve() {
	c.once.Do(func() {
		client := sts.NewFromConfig(c.cfg)
		out, err := client.GetCallerIdentity(c.ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			c.err = fmt.Errorf("failed to get caller identity: %w", err)
			return
		}
		c.account = aws.ToString(out.Account)
		c.arn = aws.ToString(out.Arn)
		c.userID = aws.ToString(out.UserId)
	})
}

// CallerIdentityData is the data exposed to templates.
type CallerIdentityData struct {
	Account string
	Arn     string
	UserID  string
}

func (c *callerIdentity) data() (*CallerIdentityData, error) {
	c.resolve()
	if c.err != nil {
		return nil, c.err
	}
	return &CallerIdentityData{
		Account: c.account,
		Arn:     c.arn,
		UserID:  c.userID,
	}, nil
}

func (c *callerIdentity) templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"caller_identity": func() (*CallerIdentityData, error) {
			return c.data()
		},
	}
}

func (c *callerIdentity) jsonnetNativeFuncs() []*jsonnet.NativeFunction {
	return []*jsonnet.NativeFunction{
		{
			Name:   "caller_identity",
			Params: jsonnetAst.Identifiers{},
			Func: func(args []any) (any, error) {
				d, err := c.data()
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"Account": d.Account,
					"Arn":     d.Arn,
					"UserID":  d.UserID,
				}, nil
			},
		},
	}
}
