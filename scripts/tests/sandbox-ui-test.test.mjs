import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const scriptPath = 'scripts/sandbox-ui-test.sh';
const source = readFileSync(scriptPath, 'utf8');
const configVariables = [
    'SANDBOX_UI_TEST_INSTRUCTIONS_PORT',
    'SANDBOX_UI_TEST_TERMINAL_PORT',
    'SANDBOX_UI_TEST_EXPIRY'
];

function runInternal(args, overrides = {}) {
    const env = { ...process.env };
    for (const name of configVariables) delete env[name];
    Object.assign(env, overrides);
    return spawnSync(scriptPath, ['internal', ...args], { encoding: 'utf8', env });
}

function requireSuccess(result) {
    assert.equal(result.status, 0, result.stderr || result.stdout);
    assert.equal(result.stderr, '');
}

function parseJson(result) {
    requireSuccess(result);
    assert.equal(result.stdout.endsWith('\n'), true);
    assert.equal(result.stdout.trim().split('\n').length, 1);
    return JSON.parse(result.stdout);
}

function enclosingFunctionPrefix(offset) {
    const openings = [...source.slice(0, offset).matchAll(/^([A-Za-z_][A-Za-z0-9_]*)\(\)\s*\{/gmu)];
    const opening = openings.at(-1);
    assert.ok(opening, `destructive Docker command is not inside a named shell function:\n${source.slice(offset, offset + 100)}`);

    const bodyStart = opening.index + opening[0].length;
    const prefix = source.slice(bodyStart, offset);
    assert.doesNotMatch(prefix, /^\s*\}/mu, 'destructive Docker command is outside the last named shell function');
    return { name: opening[1], prefix };
}

test('renders the default sandbox UI configuration', () => {
    const rendered = runInternal(['render_ui_config']);
    const config = parseJson(rendered);

    assert.deepEqual(config, {
        config: {
            version: 'v1',
            defaultTool: 'terminal',
            expiresAt: '2099-01-01T00:00:00Z',
            showTimer: false
        },
        tools: [
            {
                kind: 'instructions',
                name: 'instructions',
                url: 'http://localhost:7682'
            },
            {
                kind: 'terminal',
                name: 'terminal',
                url: 'http://localhost:7685'
            }
        ]
    });
});

test('renders overridden ports and JSON-escapes the expiry', () => {
    const expiry = '2099-07-09T12:34:56Z"quoted\\path\nnext-line\tend';
    const rendered = runInternal(['render_ui_config'], {
        SANDBOX_UI_TEST_INSTRUCTIONS_PORT: '18082',
        SANDBOX_UI_TEST_TERMINAL_PORT: '18085',
        SANDBOX_UI_TEST_EXPIRY: expiry
    });
    const config = parseJson(rendered);

    assert.equal(config.config.expiresAt, expiry);
    assert.equal(config.tools[0].url, 'http://localhost:18082');
    assert.equal(config.tools[1].url, 'http://localhost:18085');
    assert.equal(config.tools.filter(tool => tool.kind === 'instructions').length, 1);
    assert.equal(config.tools.filter(tool => tool.kind === 'terminal').length, 1);
});

test('accepts both port-number boundaries', () => {
    for (const value of ['1', '65535']) {
        const result = runInternal(['validate_port', 'TEST_PORT', value]);
        requireSuccess(result);
        assert.equal(result.stdout, '');
    }
});

test('rejects malformed and out-of-range ports with the setting name', () => {
    for (const value of ['', 'abc', '0', '65536', '70000', '80a']) {
        const result = runInternal(['validate_port', 'SANDBOX_UI_TEST_UI_PORT', value]);
        assert.notEqual(result.status, 0, `unexpectedly accepted ${JSON.stringify(value)}`);
        assert.equal(result.stdout, '');
        assert.match(result.stderr, /SANDBOX_UI_TEST_UI_PORT/u);
    }
});

test('derives deterministic fixture-scoped resource names', () => {
    const expected = new Map([
        ['net', 'p3-sandbox-ui-test-net'],
        ['site', 'p3-sandbox-ui-test-site'],
        ['creds', 'p3-sandbox-ui-test-creds'],
        ['ui', 'p3-sandbox-ui-test-ui'],
        ['terminal', 'p3-sandbox-ui-test-terminal'],
        ['instructions', 'p3-sandbox-ui-test-instructions'],
        ['ssh', 'p3-sandbox-ui-test-ssh'],
        ['generator', 'p3-sandbox-ui-test-generator'],
        ['chrome-probe', 'p3-sandbox-ui-test-chrome-probe'],
        ['creds-init', 'p3-sandbox-ui-test-creds-init'],
        ['site-init', 'p3-sandbox-ui-test-site-init'],
        ['creds-check', 'p3-sandbox-ui-test-creds-check']
    ]);

    for (const [kind, name] of expected) {
        const first = runInternal(['resource_name', kind]);
        const second = runInternal(['resource_name', kind]);
        requireSuccess(first);
        requireSuccess(second);
        assert.equal(first.stdout.trim(), name);
        assert.equal(second.stdout, first.stdout);
        assert.match(name, /^p3-sandbox-ui-test-/u);
    }
});

test('renders the terminal SSH configuration', () => {
    assert.deepEqual(parseJson(runInternal(['render_terminal_config'])), {
        targetHost: 'p3-sandbox-ui-test-ssh',
        targetUser: 'tux',
        retryInterval: '5',
        fontSize: '16'
    });
});

test('guards destructive Docker cleanup with fixture ownership checks', () => {
    const destructiveCommand = /\bdocker\s+(?:(?:container|network|volume)\s+)?rm\b/gu;
    const commands = [...source.matchAll(destructiveCommand)];
    assert.ok(commands.length > 0, 'expected fixture cleanup to contain Docker removal commands');

    for (const command of commands) {
        const { name, prefix } = enclosingFunctionPrefix(command.index);
        const checksProtection = /\bassert_removable_name\b/u.test(prefix);
        const checksOwnership = /\bassert_owned\b/u.test(prefix);
        const selectsOwnerLabel = /\blabel(?:=|\s+)[^\n]*io\.lf-certification\.p3-sandbox-ui-test\.owner\b/u.test(prefix);
        assert.ok(checksProtection, `${name} can remove a Docker resource without checking protected names`);
        assert.ok(
            checksOwnership || selectsOwnerLabel,
            `${name} can remove a Docker resource without first checking the fixture owner label`
        );
    }
});

test('retains explicit preview protection on every destructive cleanup path', () => {
    const protectedNames = [
        'ui-preview',
        'term-preview',
        'term-clipboard-review',
        'term-clipboard-final-uat',
        'instructions-preview',
        'ssh-target',
        'ttyd-preview'
    ];
    const protectedList = source.match(/^PROTECTED_NAMES=\([\s\S]*?^\s*\)/mu)?.[0];
    assert.ok(protectedList, 'expected an explicit protected resource-name list');

    const destructiveLines = source.match(/^.*\bdocker\s+(?:(?:container|network|volume)\s+)?rm\b.*$/gmu) || [];
    for (const name of protectedNames) {
        assert.match(protectedList, new RegExp(`(^|[^A-Za-z0-9_-])${name}([^A-Za-z0-9_-]|$)`, 'u'));
        for (const line of destructiveLines) assert.doesNotMatch(line, new RegExp(`\\b${name}\\b`, 'u'));
    }
});
