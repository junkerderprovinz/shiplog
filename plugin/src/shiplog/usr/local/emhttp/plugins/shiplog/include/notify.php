<?php
/* The keys in shiplog.cfg the engine built notify.json from, before the
 * settings page edited the channels through the engine. Closures instead of
 * functions, because Unraid may include a .page more than once. */

$keys = ['MATRIX_HOMESERVER', 'MATRIX_TOKEN', 'MATRIX_ROOM', 'APPRISE_URL', 'UNRAID_NOTIFY'];

/* shiplog.cfg without the lines of $remove. */
$strip = function ($cfg, array $remove) {
    $out = [];
    foreach (preg_split('/\r?\n/', $cfg) as $line) {
        $key = strstr($line, '=', true);
        if ($key === false || !in_array(trim($key), $remove, true)) {
            $out[] = $line;
        }
    }
    return implode("\n", $out);
};

return [
    'keys'  => $keys,
    'strip' => $strip,

    /* Takes the old keys out of $cfgFile once $channelsFile holds the
       channels, so the secrets they carry are kept in one place. */
    'tidy' => function ($channelsFile, $cfgFile) use ($keys, $strip) {
        if (!is_file($channelsFile) || !is_array(json_decode((string) @file_get_contents($channelsFile), true)) || !is_file($cfgFile)) {
            return;
        }
        $old = (string) @file_get_contents($cfgFile);
        $new = $strip($old, $keys);
        if ($new !== $old) {
            @file_put_contents($cfgFile, $new);
        }
    },
];
