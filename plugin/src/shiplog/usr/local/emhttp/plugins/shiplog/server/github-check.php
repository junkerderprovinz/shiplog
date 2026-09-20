<?php
/* Checks the GitHub token typed on the settings page, before Apply, and reports
 * the hourly rate limit. Without a token the result is not ok, because the
 * anonymous 60 requests run out fast and changelogs come back empty.
 * Returns {"ok":bool,"message":string}. */

header('Content-Type: application/json');

$token = isset($_GET['token']) ? trim($_GET['token']) : '';

$headers = ['User-Agent: ShipLog', 'Accept: application/vnd.github+json'];
if ($token !== '') {
    $headers[] = 'Authorization: Bearer ' . $token;
}

$ch = curl_init('https://api.github.com/rate_limit');
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_TIMEOUT        => 8,
    CURLOPT_HTTPHEADER     => $headers,
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    echo json_encode(['ok' => false, 'message' => 'Cannot reach api.github.com']);
    exit;
}
if ($code === 401) {
    echo json_encode(['ok' => false, 'message' => 'Token rejected (401), check the value.']);
    exit;
}
if ($code !== 200) {
    echo json_encode(['ok' => false, 'message' => 'GitHub returned HTTP ' . $code]);
    exit;
}

$data  = json_decode($body, true);
$limit = isset($data['resources']['core']['limit']) ? (int) $data['resources']['core']['limit']
       : (isset($data['rate']['limit']) ? (int) $data['rate']['limit'] : 0);

if ($token === '') {
    echo json_encode(['ok' => false, 'message' => 'No token set: the anonymous limit is ' . $limit . '/h, so changelogs often come back empty. Add a token to raise it to ~5000/h.']);
} elseif ($limit >= 5000) {
    echo json_encode(['ok' => true, 'message' => 'Token valid, ' . $limit . ' requests/hour.']);
} else {
    echo json_encode(['ok' => true, 'message' => 'Token accepted (' . $limit . '/h).']);
}
