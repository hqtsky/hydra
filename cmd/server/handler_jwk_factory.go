package server

import (
	"github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"github.com/ory-am/hydra/config"
	"github.com/ory/herodot"
	"github.com/ory-am/hydra/jwk"
	"github.com/square/go-jose"
	"golang.org/x/net/context"
	r "gopkg.in/gorethink/gorethink.v3"
)

func injectJWKManager(c *config.Config) {
	ctx := c.Context()

	switch con := ctx.Connection.(type) {
	case *config.MemoryConnection:
		ctx.KeyManager = &jwk.MemoryManager{}
		break
	case *config.SQLConnection:
		m := &jwk.SQLManager{
			DB: con.GetDatabase(),
			Cipher: &jwk.AEAD{
				Key: c.GetSystemSecret(),
			},
		}
		if err := m.CreateSchemas(); err != nil {
			logrus.Fatalf("Could not create jwk schema: %s", err)
		}
		ctx.KeyManager = m
		break
	case *config.RethinkDBConnection:
		con.CreateTableIfNotExists("hydra_json_web_keys")
		m := &jwk.RethinkManager{
			Session: con.GetSession(),
			Keys:    map[string]jose.JsonWebKeySet{},
			Table:   r.Table("hydra_json_web_keys"),
			Cipher: &jwk.AEAD{
				Key: c.GetSystemSecret(),
			},
		}
		if err := m.ColdStart(); err != nil {
			logrus.Fatalf("Could not fetch initial state: %s", err)
		}
		m.Watch(context.Background())
		ctx.KeyManager = m
		break
	case *config.RedisConnection:
		m := &jwk.RedisManager{
			DB: con.RedisSession(),
			Cipher: &jwk.AEAD{
				Key: c.GetSystemSecret(),
			},
		}
		ctx.KeyManager = m
		break
	default:
		logrus.Fatalf("Unknown connection type.")
	}
}

func newJWKHandler(c *config.Config, router *httprouter.Router) *jwk.Handler {
	ctx := c.Context()
	h := &jwk.Handler{
		H:        herodot.NewJSONWriter(c.Context().Logger),
		W:       ctx.Warden,
		Manager: ctx.KeyManager,
	}
	h.SetRoutes(router)
	return h
}
