<?php
/* The settings page's notification fields and the channels in notify.json the
 * engine sends through. The page edits the channels with the ids matrix,
 * apprise and unraid and leaves every other channel as it is. Closures instead
 * of functions, because Unraid may include a .page more than once. */

$keys = ['MATRIX_HOMESERVER', 'MATRIX_TOKEN', 'MATRIX_ROOM', 'APPRISE_URL', 'UNRAID_NOTIFY'];

$byId = function (array $channels, $id) {
    foreach ($channels as $c) {
        if (is_array($c) && isset($c['id']) && $c['id'] === $id) {
            return $c;
        }
    }
    return null;
};

return [
    'keys' => $keys,

    /* The values the page shows: from the channels, or from shiplog.cfg while
       there is no notify.json yet. */
    'fields' => function ($channels, array $cfg) use ($byId) {
        if (!is_array($channels)) {
            $get = function ($k) use ($cfg) { return isset($cfg[$k]) ? (string) $cfg[$k] : ''; };
            return ['hs' => $get('MATRIX_HOMESERVER'), 'token' => $get('MATRIX_TOKEN'), 'room' => $get('MATRIX_ROOM'),
                    'apprise' => $get('APPRISE_URL'), 'unraid' => $get('UNRAID_NOTIFY') === 'true'];
        }
        $m = $byId($channels, 'matrix');
        $a = $byId($channels, 'apprise');
        $u = $byId($channels, 'unraid');
        $mx = $m && isset($m['matrix']) ? $m['matrix'] : [];
        return [
            'hs'      => isset($mx['homeserver']) ? (string) $mx['homeserver'] : '',
            'token'   => isset($mx['token']) ? (string) $mx['token'] : '',
            'room'    => isset($mx['room']) ? (string) $mx['room'] : '',
            'apprise' => $a && isset($a['apprise']['url']) ? (string) $a['apprise']['url'] : '',
            'unraid'  => $u ? !empty($u['enabled']) : false,
        ];
    },

    /* The channels after the page saved $f. A channel that is only partly
       filled in stays, switched off, so nothing typed is lost. */
    'merge' => function (array $channels, array $f) use ($byId) {
        $set = function ($id, $c) use (&$channels) {
            foreach ($channels as $i => $old) {
                if (is_array($old) && isset($old['id']) && $old['id'] === $id) {
                    if ($c === null) {
                        unset($channels[$i]);
                    } else {
                        $channels[$i] = $c;
                    }
                    return;
                }
            }
            if ($c !== null) {
                $channels[] = $c;
            }
        };
        $base = function ($id, $kind) use ($byId, &$channels) {
            $c = $byId($channels, $id);
            return $c ?: ['id' => $id, 'kind' => $kind, 'enabled' => false, 'on' => 'always'];
        };

        $hs = trim($f['hs']); $token = trim($f['token']); $room = trim($f['room']);
        $m = null;
        if ($hs !== '' || $token !== '' || $room !== '') {
            $m = $base('matrix', 'matrix');
            $m['enabled'] = $hs !== '' && $token !== '' && $room !== '';
            $m['matrix'] = ['homeserver' => $hs, 'room' => $room, 'token' => $token];
        }
        $set('matrix', $m);

        $url = trim($f['apprise']);
        $a = null;
        if ($url !== '') {
            $a = $base('apprise', 'apprise');
            $a['enabled'] = (bool) preg_match('#^https?://[^/\s]+#i', $url);
            $a['apprise'] = array_merge(isset($a['apprise']) && is_array($a['apprise']) ? $a['apprise'] : [], ['url' => $url]);
        }
        $set('apprise', $a);

        $u = $byId($channels, 'unraid');
        if ($u || $f['unraid']) {
            $u = $base('unraid', 'unraid');
            $u['enabled'] = (bool) $f['unraid'];
            $set('unraid', $u);
        }
        return array_values($channels);
    },

    /* shiplog.cfg without the lines of $remove. */
    'strip' => function ($cfg, array $remove) {
        $out = [];
        foreach (preg_split('/\r?\n/', $cfg) as $line) {
            $key = strstr($line, '=', true);
            if ($key === false || !in_array(trim($key), $remove, true)) {
                $out[] = $line;
            }
        }
        return implode("\n", $out);
    },
];
