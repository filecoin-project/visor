package messages

import (
	"context"

	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

type Message struct {
	Height int64  `pg:",pk,notnull,use_zero"`
	Cid    string `pg:",pk,notnull"`

	From       string `pg:",notnull"`
	To         string `pg:",notnull"`
	Value      string `pg:"type:numeric,notnull"`
	GasFeeCap  string `pg:"type:numeric,notnull"`
	GasPremium string `pg:"type:numeric,notnull"`

	GasLimit  int64  `pg:",use_zero"`
	SizeBytes int    `pg:",use_zero"`
	Nonce     uint64 `pg:",use_zero"`
	Method    uint64 `pg:",use_zero"`
}

type MessageV0 struct {
	tableName struct{} `pg:"messages"` // nolint: structcheck,unused
	Height    int64    `pg:",pk,notnull,use_zero"`
	Cid       string   `pg:",pk,notnull"`

	From       string `pg:",notnull"`
	To         string `pg:",notnull"`
	Value      string `pg:",notnull"`
	GasFeeCap  string `pg:",notnull"`
	GasPremium string `pg:",notnull"`

	GasLimit  int64  `pg:",use_zero"`
	SizeBytes int    `pg:",use_zero"`
	Nonce     uint64 `pg:",use_zero"`
	Method    uint64 `pg:",use_zero"`
}

func (m *Message) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if m == nil {
			return (*MessageV0)(nil), true
		}

		return &MessageV0{
			Height:     m.Height,
			Cid:        m.Cid,
			From:       m.From,
			To:         m.To,
			Value:      m.Value,
			GasFeeCap:  m.GasFeeCap,
			GasPremium: m.GasPremium,
			GasLimit:   m.GasLimit,
			SizeBytes:  m.SizeBytes,
			Nonce:      m.Nonce,
			Method:     m.Method,
		}, true
	case 1:
		return m, true
	default:
		return nil, false
	}
}

func (m *Message) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vm, ok := m.AsVersion(version)
	if !ok {
		return xerrors.Errorf("Message not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vm)
}

type Messages []*Message

func (ms Messages) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(ms) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "Messages.Persist", trace.WithAttributes(label.Int("count", len(ms))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if version.Major != 1 {
		// Support older versions, but in a non-optimal way
		for _, m := range ms {
			if err := m.Persist(ctx, s, version); err != nil {
				return err
			}
		}
		return nil
	}

	return s.PersistModel(ctx, ms)
}
