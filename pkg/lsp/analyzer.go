package lsp

import (
	"context"
	"strings"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap"
)

type LexicalAnalyzer struct {
	Bindings  []LexicalBinding
	Contained []ContainedBinding
}

type ContainedBinding struct {
	Binding  bass.Symbol
	Location bass.Range

	// Bounds is not known yet
}

type LexicalBinding struct {
	Binding  bass.Symbol
	Location bass.Range
	Bounds   bass.Range
}

func (analyzer *LexicalAnalyzer) Analyze(ctx context.Context, form bass.Annotate) {
	var alreadyAnalyzed bass.Annotate
	if form.Value.Decode(&alreadyAnalyzed) == nil {
		// skip analyzing the outer annotations (range/comments/etc)
		return
	}

	var pair bass.Pair
	if err := form.Decode(&pair); err != nil {
		return
	}

	var sym bass.Symbol
	if err := pair.A.Decode(&sym); err != nil {
		return
	}

	switch sym {
	case "let":
		analyzer.analyzeLet(ctx, pair, form.Range)
	case "def":
		analyzer.analyzeDef(ctx, pair, form.Range)
	case "fn":
		analyzer.analyzeFn(ctx, pair, form.Range)
	case "defn":
		analyzer.analyzeDefn(ctx, pair, form.Range)
	case "op":
		analyzer.analyzeOp(ctx, pair, form.Range)
	case "defop":
		analyzer.analyzeDefop(ctx, pair, form.Range)
	case "provide":
		analyzer.analyzeProvide(ctx, pair, form.Range)
	}
}

func (analyzer *LexicalAnalyzer) Locate(ctx context.Context, binding bass.Symbol, params TextDocumentPositionParams) (bass.Range, bool) {
	logger := zapctx.FromContext(ctx)

	cursor := bass.Range{
		Start: bass.Position{
			Ln:  params.Position.Line + 1,
			Col: params.Position.Character,
		},
		End: bass.Position{
			Ln:  params.Position.Line + 1,
			Col: params.Position.Character,
		},
	}

	for _, b := range analyzer.Bindings {
		if b.Binding != binding {
			continue
		}

		logger := logger.With(zap.Any("loc", b.Location))

		if !cursor.IsWithin(b.Bounds) {
			logger.Debug("ignoring out of bounds lexical binding")
			continue
		}

		logger.Info("found lexical binding")
		return b.Location, true
	}

	for _, b := range analyzer.Contained {
		logger := logger.With(zap.Any("loc", b.Location))

		if b.Binding == binding {
			logger.Info("found contained binding")
			return b.Location, true
		}
	}

	return bass.Range{}, false
}

func (analyzer *LexicalAnalyzer) Complete(ctx context.Context, prefix string, params TextDocumentPositionParams) []bass.Symbol {
	logger := zapctx.FromContext(ctx)

	cursor := bass.Range{
		Start: bass.Position{
			Ln:  params.Position.Line + 1,
			Col: params.Position.Character,
		},
		End: bass.Position{
			Ln:  params.Position.Line + 1,
			Col: params.Position.Character,
		},
	}

	var bindings []bass.Symbol
	for _, b := range analyzer.Bindings {
		if !strings.HasPrefix(b.Binding.String(), prefix) {
			continue
		}

		logger := logger.With(zap.Any("loc", b.Location))

		if !cursor.IsWithin(b.Bounds) {
			logger.Debug("ignoring out of bounds lexical binding")
			continue
		}

		logger.Info("found lexical binding")

		bindings = append(bindings, b.Binding)
	}

	for _, b := range analyzer.Contained {
		logger := logger.With(zap.Any("loc", b.Location))

		if strings.HasPrefix(b.Binding.String(), prefix) {
			logger.Info("found contained binding")
			bindings = append(bindings, b.Binding)
		}
	}

	return bindings
}

func (analyzer *LexicalAnalyzer) analyzeLet(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing let")

	analyzer.contain(ctx, bounds)

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	var bindings bass.List
	if err := rest.A.Decode(&bindings); err != nil {
		logger.Error("first of rest is not a list", zap.Error(err))
		return
	}

	i := 0
	_ = bass.Each(bindings, func(v bass.Value) error {
		i++

		if i%2 == 1 {
			analyzer.analyzeBinding(ctx, v, bounds)
		}

		return nil
	})
}

func (analyzer *LexicalAnalyzer) analyzeDef(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing def")

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	analyzer.analyzeContainedBinding(ctx, rest.A)
}

func (analyzer *LexicalAnalyzer) analyzeFn(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing fn")

	analyzer.contain(ctx, bounds)

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	analyzer.analyzeBinding(ctx, rest.A, bounds)
}

func (analyzer *LexicalAnalyzer) analyzeDefn(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing defn")

	analyzer.contain(ctx, bounds)

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	analyzer.analyzeContainedBinding(ctx, rest.A)

	if err := rest.D.Decode(&rest); err != nil {
		logger.Error("first of rest is not a list", zap.Error(err))
		return
	}

	analyzer.analyzeBinding(ctx, rest.A, bounds)
}

func (analyzer *LexicalAnalyzer) analyzeOp(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing op")

	analyzer.contain(ctx, bounds)

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	// formals
	analyzer.analyzeBinding(ctx, rest.A, bounds)

	if err := rest.D.Decode(&rest); err != nil {
		logger.Error("first of rest is not a list", zap.Error(err))
		return
	}

	// scope formal
	analyzer.analyzeBinding(ctx, rest.A, bounds)
}

func (analyzer *LexicalAnalyzer) analyzeDefop(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing defop")

	analyzer.contain(ctx, bounds)

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	analyzer.analyzeContainedBinding(ctx, rest.A)

	if err := rest.D.Decode(&rest); err != nil {
		logger.Error("first of rest is not a list", zap.Error(err))
		return
	}

	// formals
	analyzer.analyzeBinding(ctx, rest.A, bounds)

	if err := rest.D.Decode(&rest); err != nil {
		logger.Error("first of rest is not a list", zap.Error(err))
		return
	}

	// scope formal
	analyzer.analyzeBinding(ctx, rest.A, bounds)
}

func (analyzer *LexicalAnalyzer) analyzeProvide(ctx context.Context, pair bass.Pair, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)
	logger.Debug("analyzing provide")

	analyzer.contain(ctx, bounds)

	var rest bass.Pair
	if err := pair.D.Decode(&rest); err != nil {
		logger.Error("rest is not a pair", zap.Error(err))
		return
	}

	analyzer.analyzeContainedBinding(ctx, rest.A)
}

func (analyzer *LexicalAnalyzer) analyzeBinding(ctx context.Context, form bass.Value, bounds bass.Range) {
	logger := zapctx.FromContext(ctx)

	var bindable bass.Bindable
	if err := form.Decode(&bindable); err != nil {
		logger.Error("form is not bindable", zap.Error(err), zap.Any("form", form))
		return
	}

	_ = bindable.EachBinding(func(binding bass.Symbol, r bass.Range) error {
		logger.Info("analyzed binding",
			zap.Any("binding", binding),
			zap.Any("bounds", bounds),
		)

		analyzer.Bindings = append(analyzer.Bindings, LexicalBinding{
			Binding:  binding,
			Location: r,
			Bounds:   bounds,
		})

		return nil
	})
}

func (analyzer *LexicalAnalyzer) analyzeContainedBinding(ctx context.Context, form bass.Value) {
	ctx, logger := zapctx.With(ctx, zap.Any("form", form))

	var bindable bass.Bindable
	if err := form.Decode(&bindable); err != nil {
		logger.Error("form is not bindable", zap.Error(err))
		return
	}

	_ = bindable.EachBinding(func(binding bass.Symbol, r bass.Range) error {
		logger.Info("recording contained binding",
			zap.Any("binding", binding),
			zap.Any("loc", r),
		)

		analyzer.Contained = append(analyzer.Contained, ContainedBinding{
			Binding:  binding,
			Location: r,
		})

		return nil
	})
}

func (analyzer *LexicalAnalyzer) contain(ctx context.Context, bounds bass.Range) {
	newContained := []ContainedBinding{}
	for _, contained := range analyzer.Contained {
		if contained.Location.IsWithin(bounds) {
			analyzer.Bindings = append(analyzer.Bindings, LexicalBinding{
				Binding:  contained.Binding,
				Location: contained.Location,
				Bounds:   bounds,
			})
		} else {
			newContained = append(newContained, contained)
		}
	}

	analyzer.Contained = newContained
}
