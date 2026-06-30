<?php
/* ShipLog — Ollama autodetect for the settings page. Probes the common local
 * Ollama endpoints from the Unraid host and returns the first reachable one plus
 * its models, so the user doesn't have to know the URL. Server-side (no CORS).
 * Returns {"ok":bool,"url":string,"models":[...],"message":string}. */

header('Content-Type: application/json');

$hint  = isset($_GET['url']) ? trim($_GET['url']) : '';
$cands = [];
if ($hint !== '' && preg_match('#^https?://#i', $hint)) {
    $cands[] = rtrim($hint, '/');
}
$cands[] = 'http://127.0.0.1:11434';
$cands[] = 'http://localhost:11434';
if (!empty($_SERVER['SERVER_ADDR'])) {
    $cands[] = 'http://' . $_SERVER['SERVER_ADDR'] . ':11434';
}
$cands[] = 'http://172.17.0.1:11434'; // docker0 gateway (Ollama as a bridge container)

$seen  = [];
$cands = array_values(array_filter($cands, function ($u) use (&$seen) {
    if (isset($seen[$u])) return false;
    $seen[$u] = 1;
    return true;
}));

foreach ($cands as $base) {
    $ch = curl_init($base . '/api/tags');
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_CONNECTTIMEOUT => 2,
        CURLOPT_TIMEOUT        => 4,
    ]);
    $body = curl_exec($ch);
    $code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);

    if ($body === false || $code !== 200) continue;
    $data = json_decode($body, true);
    if (!is_array($data) || !isset($data['models'])) continue;

    $names = [];
    foreach ($data['models'] as $m) {
        if (isset($m['name'])) $names[] = $m['name'];
    }
    echo json_encode([
        'ok'      => true,
        'url'     => $base,
        'models'  => $names,
        'message' => 'Found Ollama at ' . $base . ($names ? ' — ' . count($names) . ' model(s).' : ' — reachable, no models pulled yet.'),
    ]);
    exit;
}

echo json_encode([
    'ok'      => false,
    'url'     => '',
    'models'  => [],
    'message' => 'No Ollama found on port 11434 (tried ' . implode(', ', $cands) . '). Enter the URL manually.',
]);
