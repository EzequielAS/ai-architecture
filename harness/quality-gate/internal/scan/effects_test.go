package scan

import (
	"sort"
	"strings"
	"testing"
)

// rulesFor runs the useEffect analysis over a snippet and returns the rule ids.
func rulesFor(src string) []string {
	var out []string
	for _, f := range analyzeEffects(src) {
		out = append(out, f.Rule)
	}
	sort.Strings(out)
	return out
}

// The "bad" examples from https://react.dev/learn/you-might-not-need-an-effect,
// each expected to produce its matching rule.
func TestEffectsDetectsDocumentedAntiPatterns(t *testing.T) {
	cases := []struct {
		name string
		want string
		src  string
	}{
		{
			name: "updating state based on props or state",
			want: "effect-derives-state",
			src: `function Form({ firstName, lastName }) {
				const [fullName, setFullName] = useState('');
				useEffect(() => {
					setFullName(firstName + ' ' + lastName);
				}, [firstName, lastName]);
			}`,
		},
		{
			name: "caching expensive calculations",
			want: "effect-derives-state",
			src: `function TodoList({ todos, filter }) {
				const [visibleTodos, setVisibleTodos] = useState([]);
				useEffect(() => {
					setVisibleTodos(getFilteredTodos(todos, filter));
				}, [todos, filter]);
			}`,
		},
		{
			name: "resetting all state when a prop changes",
			want: "effect-resets-state",
			src: `function Profile({ userId }) {
				const [comment, setComment] = useState('');
				useEffect(() => {
					setComment('');
				}, [userId]);
			}`,
		},
		{
			name: "adjusting some state when a prop changes",
			want: "effect-resets-state",
			src: `function List({ items }) {
				const [selection, setSelection] = useState(null);
				useEffect(() => {
					setSelection(null);
				}, [items]);
			}`,
		},
		{
			name: "notifying parent components about state changes",
			want: "effect-notifies-parent",
			src: `function Toggle({ onChange }) {
				const [isOn, setIsOn] = useState(false);
				useEffect(() => {
					onChange(isOn);
				}, [isOn, onChange]);
			}`,
		},
		{
			name: "passing data to the parent",
			want: "effect-notifies-parent",
			src: `function Child({ onFetched }) {
				const data = useSomeAPI();
				useEffect(() => {
					if (data) {
						onFetched(data);
					}
				}, [onFetched, data]);
			}`,
		},
		{
			name: "subscribing to an external store",
			want: "effect-external-store",
			src: `function useOnlineStatus() {
				const [isOnline, setIsOnline] = useState(true);
				useEffect(() => {
					function updateState() {
						setIsOnline(navigator.onLine);
					}
					updateState();
					window.addEventListener('online', updateState);
					window.addEventListener('offline', updateState);
					return () => {
						window.removeEventListener('online', updateState);
						window.removeEventListener('offline', updateState);
					};
				}, []);
			}`,
		},
		{
			name: "fetching data without cleanup",
			want: "effect-fetch-no-cleanup",
			src: `function SearchResults({ query, page }) {
				const [results, setResults] = useState([]);
				useEffect(() => {
					fetchResults(query, page).then(json => {
						setResults(json);
					});
				}, [query, page]);
			}`,
		},
		{
			name: "chains of computations",
			want: "effect-chain",
			src: `function Game() {
				const [card, setCard] = useState(null);
				const [goldCardCount, setGoldCardCount] = useState(0);
				const [round, setRound] = useState(1);

				useEffect(() => {
					if (card !== null && card.gold) {
						setGoldCardCount(c => c + 1);
					}
				}, [card]);

				useEffect(() => {
					if (goldCardCount > 3) {
						setRound(r => r + 1);
						setGoldCardCount(0);
					}
				}, [goldCardCount]);
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules := rulesFor(tc.src)
			if !contains(rules, tc.want) {
				t.Errorf("esperava a regra %q, obteve %v", tc.want, rules)
			}
		})
	}
}

// The recommended fixes, plus legitimate Effects, must stay silent.
func TestEffectsNoFalsePositives(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "fetch with ignore-flag cleanup (documented fix)",
			src: `function SearchResults({ query, page }) {
				const [results, setResults] = useState([]);
				useEffect(() => {
					let ignore = false;
					fetchResults(query, page).then(json => {
						if (!ignore) {
							setResults(json);
						}
					});
					return () => {
						ignore = true;
					};
				}, [query, page]);
			}`,
		},
		{
			name: "fetch aborted via AbortController",
			src: `function User({ id }) {
				const [user, setUser] = useState(null);
				useEffect(() => {
					const controller = new AbortController();
					fetch('/api/users/' + id, { signal: controller.signal })
						.then(r => r.json())
						.then(setUser);
					return () => controller.abort();
				}, [id]);
			}`,
		},
		{
			name: "synchronizing with the DOM",
			src: `function Title({ title }) {
				useEffect(() => {
					document.title = title;
				}, [title]);
			}`,
		},
		{
			name: "UI event listener is a valid Effect",
			src: `function Resizer() {
				const [width, setWidth] = useState(0);
				useEffect(() => {
					function handleResize() {
						setWidth(window.innerWidth);
					}
					window.addEventListener('resize', handleResize);
					return () => window.removeEventListener('resize', handleResize);
				}, []);
			}`,
		},
		{
			name: "observable pushing values is not an external store read",
			src: `function Feed() {
				const [data, setData] = useState(null);
				useEffect(() => {
					const sub = source.subscribe(value => setData(value));
					return () => sub.unsubscribe();
				}, []);
			}`,
		},
		{
			name: "one-time mount flag has no dependencies",
			src: `function App() {
				const [mounted, setMounted] = useState(false);
				useEffect(() => {
					setMounted(true);
				}, []);
			}`,
		},
		{
			name: "measuring the DOM through a ref",
			src: `function Box({ items }) {
				const ref = useRef(null);
				const [height, setHeight] = useState(0);
				useEffect(() => {
					if (!ref.current) return;
					setHeight(ref.current.offsetHeight);
				}, [items]);
			}`,
		},
		{
			name: "timers are not state setters",
			src: `function Splash({ delay }) {
				useEffect(() => {
					setTimeout(done, delay);
				}, [delay]);
			}`,
		},
		{
			name: "side effect that is not a state update",
			src: `function Page({ id }) {
				useEffect(() => {
					analytics.track('view', { id });
				}, [id]);
			}`,
		},
		{
			name: "identifier merely starting with on",
			src: `function Once({ x }) {
				useEffect(() => {
					onlyRunSomething(x);
				}, [x]);
			}`,
		},
		{
			name: "setter unrelated to the dependencies",
			src: `function Widget({ id }) {
				const [state, setState] = useState({});
				useEffect(() => {
					setState(s => ({ ...s, loading: true }));
				}, [id]);
			}`,
		},
		{
			name: "effects in different components are not a chain",
			src: `function A() {
				const [count, setCount] = useState(0);
				useEffect(() => {
					setCount(compute());
				}, [source]);
			}
			function B({ count }) {
				const [label, setLabel] = useState('');
				useEffect(() => {
					track(count);
					setLabel(String(count));
				}, [count]);
			}`,
		},
		{
			name: "no effect at all",
			src:  `export const sum = (a, b) => a + b;`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rules := rulesFor(tc.src); len(rules) > 0 {
				t.Errorf("falso positivo: %v", rules)
			}
		})
	}
}

func TestEffectsReportsLineAndMessage(t *testing.T) {
	src := "function Form({ firstName, lastName }) {\n" +
		"  const [fullName, setFullName] = useState('');\n" +
		"  useEffect(() => {\n" +
		"    setFullName(firstName + ' ' + lastName);\n" +
		"  }, [firstName, lastName]);\n" +
		"}\n"

	findings := analyzeEffects(src)
	if len(findings) != 1 {
		t.Fatalf("esperava 1 finding, obteve %d (%v)", len(findings), findings)
	}
	if findings[0].Line != 3 {
		t.Errorf("esperava linha 3, obteve %d", findings[0].Line)
	}
	if !strings.Contains(findings[0].Message, "renderiza") {
		t.Errorf("mensagem sem orientação: %q", findings[0].Message)
	}
}

// React.useEffect must be recognised like the bare import.
func TestEffectsNamespacedCall(t *testing.T) {
	src := `function C({ a, b }) {
		const [sum, setSum] = useState(0);
		React.useEffect(() => {
			setSum(a + b);
		}, [a, b]);
	}`
	if rules := rulesFor(src); !contains(rules, "effect-derives-state") {
		t.Errorf("esperava effect-derives-state, obteve %v", rules)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
