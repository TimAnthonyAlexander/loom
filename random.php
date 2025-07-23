<?php
// Random PHP code
$name = "Alice";
$age = random_int(18, 65);
function greet($name) {
    global $age;
    echo "Hello, $name! You are $age years old.\n";
}
$arr = [1, 2, 3, 4, 5];
foreach ($arr as $num) {
    echo "Number: $num\n";
}
class Random {
    private $value;
    public function __construct($val) { $this->value = $val; }
    public function show() { echo "Value: {$this->value}\n"; }
}
$randObj = new Random(random_int(1, 100));
greet($name);
$randObj->show();
?>