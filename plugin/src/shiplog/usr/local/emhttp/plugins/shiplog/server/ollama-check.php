<?php
/* ShipLog — live Ollama check for the settings page. Tests the URL + model the
 * user just typed (before Apply), straight from PHP so there's no CORS and no
 * dependency on the engine. Returns {"ok":bool,"message":string}. */

header('Content-Type: application/json');

$url   = isset($_GET['url'])   ? trim($_GET['url'])   : '';
$model = isset($_GET['model']) ? trim($_GET['model']) : '';

if ($url === '' || $model === '') {
    echo json_encode(['ok' => false, 'message' => 'Enter the Ollama URL and model first.']);
    exit;
}
if (!preg_match('#^https?://#i', $url)) {
    echo json_encode(['ok' => false, 'message' => 'URL must start with http:// or https://']);
    exit;
}
$url = rtrim($url, '/');

$ch = curl_init($url . '/api/tags');
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_TIMEOUT        => 8,
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    echo json_encode(['ok' => false, 'message' => 'Cannot reach Ollama at ' . $url]);
    exit;
}
if ($code !== 200) {
    echo json_encode(['ok' => false, 'message' => 'Ollama returned HTTP ' . $code]);
    exit;
}

$data   = json_decode($body, true);
$models = (is_array($data) && isset($data['models'])) ? $data['models'] : [];
$names  = [];
foreach ($models as $m) {
    if (isset($m['name'])) {
        $names[] = $m['name'];
    }
}
$found = false;
foreach ($names as $n) {
    if ($n === $model || strpos($n, $model . ':') === 0) {
        $found = true;
        break;
    }
}

if ($found) {
    echo json_encode(['ok' => true, 'message' => 'Reachable, model "' . $model . '" found.']);
} else {
    $avail = $names ? implode(', ', array_slice($names, 0, 8)) : '(none)';
    echo json_encode(['ok' => false, 'message' => 'Reachable, but model "' . $model . '" not found. Available: ' . $avail]);
}
