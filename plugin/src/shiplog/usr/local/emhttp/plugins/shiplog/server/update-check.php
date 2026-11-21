<?php
/* Runs the update check of Unraid's Plugins tab for ShipLog alone and answers
 * with where ShipLog's updates stand, as include/update.php reads them.
 * A failed check answers {"kind":"failed","reason":...} with plugin's own words.
 * It only runs on a POST, which Unraid checks for its CSRF token. */

header('Content-Type: application/json');

if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    http_response_code(405);
    echo json_encode(['kind' => 'failed', 'reason' => 'Ask in a POST.']);
    exit;
}

exec('/usr/local/sbin/plugin check shiplog.plg 2>&1', $out, $rc);
if ($rc !== 0) {
    $why = '';
    foreach ($out as $line) {
        if (preg_match('/^plugin: (?!checking:)(.+)/', $line, $m)) $why = trim($m[1]);
    }
    echo json_encode(['kind' => 'failed', 'reason' => $why]);
    exit;
}
$state = include '/usr/local/emhttp/plugins/shiplog/include/update.php';
echo json_encode($state());
