<?php
/* ShipLog — live Docker Hub credential check for the settings page. Tests the
 * username + token the user just typed (before Apply) by asking Docker Hub's
 * registry auth endpoint for a token with HTTP Basic auth — the exact mechanism
 * the resolver uses. Valid creds → 200 + token; bad creds → 401. Returns
 * {"ok":bool,"message":string}. */

header('Content-Type: application/json');

$user  = isset($_GET['user'])  ? trim($_GET['user'])  : '';
$token = isset($_GET['token']) ? trim($_GET['token']) : '';

if ($user === '' || $token === '') {
    echo json_encode(['ok' => false, 'message' => 'Enter the Docker Hub username and token first.']);
    exit;
}

$url = 'https://auth.docker.io/token?service=registry.docker.io&scope='
     . rawurlencode('repository:library/hello-world:pull');

$ch = curl_init($url);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_CONNECTTIMEOUT => 4,
    CURLOPT_TIMEOUT        => 10,
    CURLOPT_HTTPAUTH       => CURLAUTH_BASIC,
    CURLOPT_USERPWD        => $user . ':' . $token,
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    echo json_encode(['ok' => false, 'message' => 'Cannot reach Docker Hub auth.']);
    exit;
}
if ($code === 401 || $code === 403) {
    echo json_encode(['ok' => false, 'message' => 'Docker Hub rejected these credentials (HTTP ' . $code . ').']);
    exit;
}
if ($code !== 200) {
    echo json_encode(['ok' => false, 'message' => 'Docker Hub auth returned HTTP ' . $code . '.']);
    exit;
}

$data = json_decode($body, true);
if (is_array($data) && (!empty($data['token']) || !empty($data['access_token']))) {
    echo json_encode(['ok' => true, 'message' => 'Credentials accepted by Docker Hub.']);
} else {
    echo json_encode(['ok' => false, 'message' => 'Unexpected response from Docker Hub auth.']);
}
