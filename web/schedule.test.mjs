// Self-check for the UTC→local time math in static/js/schedule.js. Run: node web/schedule.test.mjs
// Lives outside static/ so it is not embedded into the server binary.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('./static/js/schedule.js', import.meta.url), 'utf8');
const { localizeTask, isRunning, formatSpan, formatWindow } = vm.runInThisContext(`${source}\n({ localizeTask, isRunning, formatSpan, formatWindow })`);

// Switch the process timezone (Node applies TZ changes at runtime); Etc/GMT signs are inverted.
function inZone(tz, fn) {
    const saved = process.env.TZ;
    process.env.TZ = tz;
    try { fn(); } finally { if (saved === undefined) delete process.env.TZ; else process.env.TZ = saved; }
}
const SUN = 0, MON = 1, TUE = 2, THU = 4;

inZone('Etc/GMT-3', () => { // UTC+3: late UTC evening moves to the next local day
    const loc = localizeTask({ time: '22:30', days_of_week: 'Mon,Wed', is_recurring: true });
    assert.equal(loc.start, 90);
    assert.deepEqual(loc.days, [TUE, THU]);
});

inZone('Etc/GMT+5', () => { // UTC-5: early UTC morning moves to the previous local day
    const loc = localizeTask({ time: '02:00', days_of_week: 'Mon', is_recurring: true });
    assert.equal(loc.start, 21 * 60);
    assert.deepEqual(loc.days, [SUN]);

    // 03:00 UTC Mon for 4h = Sun 22:00 to Mon 02:00 local, crossing midnight.
    const night = localizeTask({ time: '03:00', duration: '04:00', days_of_week: 'Mon', is_recurring: true });
    assert.deepEqual([night.start, night.end, night.days], [22 * 60, 26 * 60, [SUN]]);
    const monday = (h, m) => new Date(2026, 10, 2, h, m); // 2 Nov 2026 is a Monday
    assert.equal(monday(0, 30).getDay(), MON);
    assert.equal(isRunning(night, monday(0, 30)), true, 'window that started yesterday is running');
    assert.equal(isRunning(night, monday(2, 0)), false, 'end is exclusive');
    assert.equal(isRunning(night, monday(21, 59)), false);
});

inZone('Etc/GMT-3', () => { // Dated one-off: 22:30 UTC on 5 Nov is 01:30 local on 6 Nov
    const loc = localizeTask({ time: '22:30', duration: '01:00', days_of_week: '', is_recurring: false, date: '2026-11-05' });
    assert.equal(loc.startDate.getTime(), Date.UTC(2026, 10, 5, 22, 30));
    assert.deepEqual([loc.startDate.getDate(), loc.startDate.getHours(), loc.start], [6, 1, 90]);
    assert.equal(loc.endDate.getTime() - loc.startDate.getTime(), 3600000);
    assert.deepEqual(loc.days, []);
    assert.equal(isRunning(loc, loc.startDate), true);
    assert.equal(isRunning(loc, loc.endDate), false);
    assert.equal(isRunning(loc, new Date(loc.startDate.getTime() - 7 * 86400000)), false, 'a week earlier is not running');

    assert.equal(localizeTask({ time: '10:00', is_recurring: false }).startDate, undefined, 'undated one-off has no date');
    assert.equal(localizeTask({ time: '10:00', is_recurring: false, date: '5 Nov' }).startDate, undefined, 'malformed date is ignored');
});

assert.equal(formatSpan(26 * 60), '26:00');
assert.equal(formatSpan(168 * 60), '168:00');
assert.equal(formatWindow({ start: 9 * 60, end: 9 * 60 + 50 * 60, duration: 50 * 60 }), '09:00–11:00 (ends 2 days later)');
assert.equal(formatWindow({ start: 22 * 60, end: 26 * 60, duration: 240 }), '22:00–02:00');

console.log('schedule.js self-check passed');
