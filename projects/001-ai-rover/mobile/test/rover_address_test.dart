import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/rover_address.dart';

void main() {
  group('RoverAddress.parse', () {
    test('acepta IP y nombre mDNS', () {
      expect(RoverAddress.parse('192.168.1.50').host, '192.168.1.50');
      expect(RoverAddress.parse('  Rover.local ').host, 'rover.local');
    });

    test('quita http://, el puerto 80 y la ruta', () {
      expect(RoverAddress.parse('http://rover.local/').host, 'rover.local');
      expect(
        RoverAddress.parse('HTTP://192.168.1.50:80/status').host,
        '192.168.1.50',
      );
    });

    test('construye las URL del firmware', () {
      final address = RoverAddress.parse('192.168.1.50');
      expect(address.statusUri.toString(), 'http://192.168.1.50/status');
      expect(address.streamUri.toString(), 'http://192.168.1.50:81/stream');
    });

    for (final input in [
      '',
      '   ',
      'rover local',
      'https://rover.local',
      'ftp://rover.local',
      'rover.local:8080',
      'user@rover.local',
      'rover_.local',
      '-rover.local',
    ]) {
      test('rechaza "$input"', () {
        expect(() => RoverAddress.parse(input), throwsFormatException);
      });
    }
  });
}
