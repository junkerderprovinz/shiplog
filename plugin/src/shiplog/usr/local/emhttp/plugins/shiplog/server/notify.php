<?php
/* Saves the settings page's notification fields to notify.json, then takes the
 * old keys out of shiplog.cfg and restarts the engine, which reads the channels
 * at start. Unraid checks the csrf_token of a POST before this script runs.
 *
 *   POST notify.php  unraid=true|false, hs, token, room, apprise
 */

$dir  = '/boot/config/plugins/shiplog';
$file = "$dir/notify.json";
$cfg  = "$dir/shiplog.cfg";
$N    = include '/usr/local/emhttp/plugins/shiplog/include/notify.php';

$answer = function ($code, $body) {
    http_response_code($code);
    header('Content-Type: application/json');
    echo json_encode($body);
    exit;
};

if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    $answer(405, ['error' => 'not allowed']);
}

$channels = [];
if (is_file($file)) {
    $channels = json_decode((string) @file_get_contents($file), true);
    if (!is_array($channels)) {
        $answer(409, ['error' => 'notify.json cannot be read']);
    }
}
$post = function ($k) { return isset($_POST[$k]) ? (string) $_POST[$k] : ''; };
$channels = $N['merge']($channels, [
    'unraid' => $post('unraid') === 'true',
    'hs' => $post('hs'), 'token' => $post('token'), 'room' => $post('room'),
    'apprise' => $post('apprise'),
]);

@mkdir($dir, 0700, true);
$tmp  = "$file.tmp";
$json = json_encode($channels, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE) . "\n";
if (file_put_contents($tmp, $json) === false || !rename($tmp, $file)) {
    @unlink($tmp);
    $answer(500, ['error' => 'notify.json could not be written']);
}

// The old keys go only once the channels are safe in notify.json.
if (is_file($cfg)) {
    $old = (string) file_get_contents($cfg);
    $new = $N['strip']($old, $N['keys']);
    if ($new !== $old) {
        file_put_contents($cfg, $new);
    }
}

exec('/usr/local/emhttp/plugins/shiplog/scripts/rc.shiplog restart >/dev/null 2>&1');
$answer(200, ['ok' => true]);
