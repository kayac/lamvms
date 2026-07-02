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
	once    sync.Once
	account string
	arn     string
	userID  string
	err     error
}

func newCallerIdentity(cfg aws.Config) *callerIdentity {
	return &callerIdentity{cfg: cfg}
}

func (c *callerIdentity) resolve(ctx context.Context) {
	c.once.Do(func() {
		client := sts.NewFromConfig(c.cfg)
		out, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
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

func (c *callerIdentity) data(ctx context.Context) (*CallerIdentityData, error) {
	c.resolve(ctx)
	if c.err != nil {
		return nil, c.err
	}
	return &CallerIdentityData{
		Account: c.account,
		Arn:     c.arn,
		UserID:  c.userID,
	}, nil
}

func (c *callerIdentity) templateFuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"caller_identity": func() (*CallerIdentityData, error) {
			return c.data(ctx)
		},
	}
}

func (c *callerIdentity) jsonnetNativeFuncs(ctx context.Context) []*jsonnet.NativeFunction {
	return []*jsonnet.NativeFunction{
		{
			Name:   "caller_identity",
			Params: jsonnetAst.Identifiers{},
			Func: func(args []any) (any, error) {
				d, err := c.data(ctx)
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
