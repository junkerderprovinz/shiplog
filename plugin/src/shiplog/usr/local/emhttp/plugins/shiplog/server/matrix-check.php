<?php
/* ShipLog — live Matrix check for the settings page. Tests the homeserver +
 * token (whoami) and, if the room is an #alias, that it resolves. Straight from
 * PHP so there's no CORS. Returns {"ok":bool,"message":string}. */

header('Content-Type: application/json');

$hs    = isset($_GET['hs'])    ? trim($_GET['hs'])    : '';
$token = isset($_GET['token']) ? trim($_GET['token']) : '';
$room  = isset($_GET['room'])  ? trim($_GET['room'])  : '';

if ($hs === '' || $token === '') {
    echo json_encode(['ok' => false, 'message' => 'Enter the homeserver and token first.']);
    exit;
}
if (!preg_match('#^https?://#i', $hs)) {
    echo json_encode(['ok' => false, 'message' => 'Homeserver must start with http:// or https://']);
    exit;
}
$hs = rtrim($hs, '/');

$ch = curl_init($hs . '/_matrix/client/v3/account/whoami');
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_TIMEOUT        => 8,
    CURLOPT_HTTPHEADER     => ['Authorization: Bearer ' . $token],
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    echo json_encode(['ok' => false, 'message' => 'Cannot reach ' . $hs]);
    exit;
}
if ($code === 401) {
    echo json_encode(['ok' => false, 'message' => 'Token rejected (401).']);
    exit;
}
if ($code !== 200) {
    echo json_encode(['ok' => false, 'message' => 'whoami returned HTTP ' . $code]);
    exit;
}
$who  = json_decode($body, true);
$user = (is_array($who) && isset($who['user_id'])) ? $who['user_id'] : '(unknown user)';

// If the room is an alias, confirm it resolves (room ids can't be checked cheaply).
if ($room !== '' && $room[0] === '#') {
    $ch = curl_init($hs . '/_matrix/client/v3/directory/room/' . rawurlencode($room));
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_TIMEOUT        => 8,
        CURLOPT_HTTPHEADER     => ['Authorization: Bearer ' . $token],
    ]);
    curl_exec($ch);
    $rc = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    if ($rc !== 200) {
        echo json_encode(['ok' => false, 'message' => 'Token OK (' . $user . '), but room alias "' . $room . '" did not resolve.']);
        exit;
    }
}

echo json_encode(['ok' => true, 'message' => 'Token OK — ' . $user]);
