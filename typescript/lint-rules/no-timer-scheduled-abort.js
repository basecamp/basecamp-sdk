/**
 * oxlint JS plugin: no-timer-scheduled-abort.
 *
 * Forbids scheduling an `abort()` on a wall-clock timer inside
 * `typescript/tests/`. A test that arms `setTimeout(() => controller.abort(), N)`
 * and races it against a mocked response is asserting machine load, not
 * behavior: it passes when the scheduler is prompt and reds when it is not, on
 * somebody else's unrelated PR.
 *
 * Abort from a seam that proves the request is already in flight instead —
 * inside the MSW handler, or from the retry hook the loop fires immediately
 * before it sleeps. See CONTRIBUTING.md, "Never schedule a cancellation on a
 * wall clock".
 *
 * ## Why this rule exists at all
 *
 * #655 fixed one instance of this shape and declared the class contained,
 * scoping itself with an `rg` typed into an issue body that required a
 * NO-ARGUMENT `abort()`. Two instances passing a reason were live in
 * `middleware-lifecycle.test.ts` at that moment and shipped anyway; one of them
 * went red in CI two months later (#783). There was no instrument to reassess,
 * so this is the first one.
 *
 * ## What it recognizes
 *
 * A call to something named `abort` — `x.abort(…)`, `x["abort"](…)`, or a bare
 * `abort(…)` — lexically inside a function passed as the FIRST argument of
 * `setTimeout` / `setInterval`, spelled bare or as a member access
 * (`globalThis.setTimeout`, `window.setTimeout`). Nesting is followed: an abort
 * inside a function inside the timer callback still reports, because the timer
 * is still what schedules it.
 *
 * ## What it cannot see
 *
 * State this honestly rather than claiming every call site (AGENTS.md):
 *
 *   - a timer function reached under another name — `const later = setTimeout`,
 *     an import alias, a wrapper such as `delay(() => c.abort(), 50)`;
 *   - a callback passed by reference rather than written inline —
 *     `setTimeout(cancel, 50)` where `cancel` aborts;
 *   - a computed abort access whose key is not a literal — `c[method]()`;
 *   - anything outside `typescript/tests/`, which is deliberate: production
 *     code schedules aborts on timers legitimately (`src/client.ts`,
 *     `src/download.ts`, `src/oauth/*`) and those are request timeouts, not
 *     test assertions.
 *
 * So this holds a SYNTACTIC invariant across the call sites its matcher
 * recognizes, including ones not written yet. It does not observe what a test
 * does at runtime, and it is not the whole guarantee — the population bound is
 * `rg -n '\.abort\(' typescript/tests`, and each site should be classified by
 * what makes the abort land. At the time this rule was written that was 14 call
 * sites: stubbed sleep or fetch 5, MSW handler 3, retry hook 3, already aborted
 * before the call 2, synchronous after dispatch 1. None timer-scheduled.
 *
 * ## Suppressing it
 *
 * A test whose SUBJECT is a caller's own timer-driven abort is a legitimate
 * exception. Suppress at the site, with the reason, so a reader can evaluate
 * it — never widen the matcher to be blind to a shape:
 *
 *     // oxlint-disable-next-line basecamp-tests/no-timer-scheduled-abort -- why
 *
 * ## Fail-closed
 *
 * oxlint's JS plugin API is alpha and explicitly not covered by semver, and
 * `oxlint` here is a caret range. If a bump changed the API such that this rule
 * silently stopped matching, the gate would pass while enforcing nothing.
 * `lint-rules/test-no-timer-scheduled-abort.mjs` is what makes that loud: it
 * asserts the rule still FIRES on the two forms this repo removed, so an API
 * break fails the build instead of disarming it. Run it wherever this rule
 * runs.
 */

const TIMER_NAMES = new Set(["setTimeout", "setInterval"]);

/** `setTimeout(…)`, `globalThis.setTimeout(…)`, `window.setTimeout(…)`. */
function isTimerCall(node) {
  const callee = node.callee;
  if (callee.type === "Identifier") return TIMER_NAMES.has(callee.name);
  if (callee.type === "MemberExpression" && !callee.computed && callee.property.type === "Identifier") {
    return TIMER_NAMES.has(callee.property.name);
  }
  return false;
}

/** `x.abort(…)`, `x["abort"](…)`, bare `abort(…)`. */
function isAbortCall(node) {
  const callee = node.callee;
  if (callee.type === "Identifier") return callee.name === "abort";
  if (callee.type !== "MemberExpression") return false;
  if (!callee.computed) {
    return callee.property.type === "Identifier" && callee.property.name === "abort";
  }
  return callee.property.type === "Literal" && callee.property.value === "abort";
}

function isFunction(node) {
  return (
    node.type === "ArrowFunctionExpression" ||
    node.type === "FunctionExpression" ||
    node.type === "FunctionDeclaration"
  );
}

const rule = {
  meta: {
    type: "problem",
    docs: {
      description: "Disallow scheduling an abort() on a wall-clock timer in tests",
    },
    schema: [],
  },
  create(context) {
    return {
      CallExpression(node) {
        if (!isAbortCall(node)) return;

        // Walk UP rather than descending from the timer: the question is
        // "what schedules this abort?", and the ancestor chain answers it
        // directly at any nesting depth without a hand-rolled subtree walk.
        for (let current = node.parent; current; current = current.parent) {
          if (!isFunction(current)) continue;
          const parent = current.parent;
          if (
            parent &&
            parent.type === "CallExpression" &&
            isTimerCall(parent) &&
            parent.arguments[0] === current
          ) {
            context.report({
              node,
              message:
                "Do not schedule abort() on a wall-clock timer in a test — it races machine " +
                "load. Abort from a seam that proves the request is in flight (inside the MSW " +
                "handler, or from the retry hook fired immediately before the backoff sleep). " +
                "See CONTRIBUTING.md, 'Never schedule a cancellation on a wall clock'.",
            });
            return;
          }
        }
      },
    };
  },
};

export default {
  meta: { name: "basecamp-tests" },
  rules: { "no-timer-scheduled-abort": rule },
};
