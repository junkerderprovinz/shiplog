<?php
/* Sends a test message to the Apprise API notify URL typed on the settings
 * page. The URL comes by POST, so its key stays out of query strings and logs,
 * and answers name the server only. Returns {"ok":bool,"message":string}. */

header('Content-Type: application/json');

$target = isset($_POST['url']) ? trim($_POST['url']) : '';
if ($target === '') {
    echo json_encode(['ok' => false, 'message' => 'Enter the Apprise URL first.']);
    exit;
}
$u = parse_url($target);
if (!$u || !isset($u['scheme'], $u['host']) || !in_array(strtolower($u['scheme']), ['http', 'https'], true)) {
    echo json_encode(['ok' => false, 'message' => 'The Apprise URL must start with http:// or https://']);
    exit;
}
$where = strtolower($u['scheme']) . '://' . $u['host'] . (isset($u['port']) ? ':' . $u['port'] : '');

$ch = curl_init($target);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_POST           => true,
    CURLOPT_POSTFIELDS     => json_encode(['body' => "🚢 ShipLog · test message from the settings page", 'type' => 'info', 'format' => 'text']),
    CURLOPT_HTTPHEADER     => ['Content-Type: application/json'],
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_TIMEOUT        => 15,
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    echo json_encode(['ok' => false, 'message' => 'Cannot reach ' . $where]);
} elseif ($code === 200) {
    echo json_encode(['ok' => true, 'message' => 'Test message sent through ' . $where]);
} elseif ($code === 404) {
    echo json_encode(['ok' => false, 'message' => 'Nothing is saved under this key on ' . $where . ' (404).']);
} elseif ($code === 424) {
    echo json_encode(['ok' => false, 'message' => 'Apprise could not deliver to at least one service (424).']);
} else {
    echo json_encode(['ok' => false, 'message' => 'Apprise returned HTTP ' . $code]);
}
