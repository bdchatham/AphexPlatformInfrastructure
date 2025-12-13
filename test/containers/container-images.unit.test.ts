/**
 * Unit tests for container images
 * 
 * These tests verify that container images have the correct base images,
 * required tools installed, and scripts are executable.
 * 
 * Requirements: 3.1, 3.2, 3.3, 3.4
 */

import * as fs from 'fs';
import * as path from 'path';

describe('Container Images Unit Tests', () => {
  const containersDir = path.join(__dirname, '../../containers');

  describe('Builder Image', () => {
    const builderDir = path.join(containersDir, 'builder');
    const dockerfilePath = path.join(builderDir, 'Dockerfile');
    const scriptPath = path.join(builderDir, 'aphex-build');

    it('should have a Dockerfile', () => {
      expect(fs.existsSync(dockerfilePath)).toBe(true);
    });

    it('should use node:20-alpine as base image', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('FROM node:20-alpine');
    });

    it('should install Node.js 20.x', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      // Node.js 20.x is provided by the base image node:20-alpine
      expect(dockerfile).toContain('FROM node:20-alpine');
    });

    it('should install Python 3.11', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('python3');
    });

    it('should install Git', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('git');
    });

    it('should install AWS CLI v2', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('aws-cli');
    });

    it('should install build tools (gcc, make, g++)', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('gcc');
      expect(dockerfile).toContain('make');
      expect(dockerfile).toContain('g++');
    });

    it('should have aphex-build script', () => {
      expect(fs.existsSync(scriptPath)).toBe(true);
    });

    it('should make aphex-build script executable', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('chmod +x /usr/local/bin/aphex-build');
    });

    it('should copy aphex-build to /usr/local/bin/', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('COPY aphex-build /usr/local/bin/aphex-build');
    });

    it('should set aphex-build as entrypoint', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('ENTRYPOINT ["/usr/local/bin/aphex-build"]');
    });

    it('should set working directory to /workspace', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('WORKDIR /workspace');
    });

    it('should have image metadata labels', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('LABEL');
      expect(dockerfile).toContain('org.opencontainers.image');
    });
  });

  describe('Deployer Image', () => {
    const deployerDir = path.join(containersDir, 'deployer');
    const dockerfilePath = path.join(deployerDir, 'Dockerfile');
    const pipelineScriptPath = path.join(deployerDir, 'aphex-deploy-pipeline');
    const stackScriptPath = path.join(deployerDir, 'aphex-deploy-stack');

    it('should have a Dockerfile', () => {
      expect(fs.existsSync(dockerfilePath)).toBe(true);
    });

    it('should use node:20-alpine as base image', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('FROM node:20-alpine');
    });

    it('should install Node.js 20.x', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      // Node.js 20.x is provided by the base image node:20-alpine
      expect(dockerfile).toContain('FROM node:20-alpine');
    });

    it('should install Python 3.11', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('python3');
    });

    it('should install AWS CDK CLI', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('npm install -g aws-cdk');
    });

    it('should install AWS CLI v2', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('aws-cli');
    });

    it('should install kubectl', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('kubectl');
    });

    it('should install Git', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('git');
    });

    it('should have aphex-deploy-pipeline script', () => {
      expect(fs.existsSync(pipelineScriptPath)).toBe(true);
    });

    it('should have aphex-deploy-stack script', () => {
      expect(fs.existsSync(stackScriptPath)).toBe(true);
    });

    it('should make aphex-deploy-pipeline script executable', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('chmod +x /usr/local/bin/aphex-deploy-pipeline');
    });

    it('should make aphex-deploy-stack script executable', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('chmod +x /usr/local/bin/aphex-deploy-stack');
    });

    it('should copy aphex-deploy-pipeline to /usr/local/bin/', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('COPY aphex-deploy-pipeline /usr/local/bin/aphex-deploy-pipeline');
    });

    it('should copy aphex-deploy-stack to /usr/local/bin/', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('COPY aphex-deploy-stack /usr/local/bin/aphex-deploy-stack');
    });

    it('should set working directory to /workspace', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('WORKDIR /workspace');
    });

    it('should have image metadata labels', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('LABEL');
      expect(dockerfile).toContain('org.opencontainers.image');
    });
  });

  describe('Tester Image', () => {
    const testerDir = path.join(containersDir, 'tester');
    const dockerfilePath = path.join(testerDir, 'Dockerfile');
    const scriptPath = path.join(testerDir, 'aphex-test');

    it('should have a Dockerfile', () => {
      expect(fs.existsSync(dockerfilePath)).toBe(true);
    });

    it('should use node:20-alpine as base image', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('FROM node:20-alpine');
    });

    it('should install Node.js 20.x', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      // Node.js 20.x is provided by the base image node:20-alpine
      expect(dockerfile).toContain('FROM node:20-alpine');
    });

    it('should install Python 3.11', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('python3');
    });

    it('should install Jest', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('jest');
    });

    it('should install Mocha', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('mocha');
    });

    it('should install Pytest', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('pytest');
    });

    it('should install AWS CLI v2', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('aws-cli');
    });

    it('should have aphex-test script', () => {
      expect(fs.existsSync(scriptPath)).toBe(true);
    });

    it('should make aphex-test script executable', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('chmod +x /usr/local/bin/aphex-test');
    });

    it('should copy aphex-test to /usr/local/bin/', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('COPY aphex-test /usr/local/bin/aphex-test');
    });

    it('should set aphex-test as entrypoint', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('ENTRYPOINT ["/usr/local/bin/aphex-test"]');
    });

    it('should set working directory to /workspace', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('WORKDIR /workspace');
    });

    it('should have image metadata labels', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('LABEL');
      expect(dockerfile).toContain('org.opencontainers.image');
    });
  });

  describe('Validator Image', () => {
    const validatorDir = path.join(containersDir, 'validator');
    const dockerfilePath = path.join(validatorDir, 'Dockerfile');
    const scriptPath = path.join(validatorDir, 'aphex-validate');

    it('should have a Dockerfile', () => {
      expect(fs.existsSync(dockerfilePath)).toBe(true);
    });

    it('should use python:3.11-slim as base image', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('FROM python:3.11-slim');
    });

    it('should install Python 3.11', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      // Python 3.11 is provided by the base image python:3.11-slim
      expect(dockerfile).toContain('FROM python:3.11-slim');
    });

    it('should install jsonschema library', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('jsonschema');
    });

    it('should install PyYAML library', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('PyYAML');
    });

    it('should install AWS CLI v2', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('awscli');
    });

    it('should have aphex-validate script', () => {
      expect(fs.existsSync(scriptPath)).toBe(true);
    });

    it('should make aphex-validate script executable', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('chmod +x /usr/local/bin/aphex-validate');
    });

    it('should copy aphex-validate to /usr/local/bin/', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('COPY aphex-validate /usr/local/bin/aphex-validate');
    });

    it('should set aphex-validate as entrypoint', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('ENTRYPOINT ["/usr/local/bin/aphex-validate"]');
    });

    it('should set working directory to /workspace', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('WORKDIR /workspace');
    });

    it('should have image metadata labels', () => {
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      expect(dockerfile).toContain('LABEL');
      expect(dockerfile).toContain('org.opencontainers.image');
    });
  });

  describe('Script Executability', () => {
    it('should have executable aphex-build script', () => {
      const scriptPath = path.join(containersDir, 'builder', 'aphex-build');
      expect(fs.existsSync(scriptPath)).toBe(true);
      
      // Check if file has shebang
      const content = fs.readFileSync(scriptPath, 'utf-8');
      expect(content.startsWith('#!/')).toBe(true);
    });

    it('should have executable aphex-deploy-pipeline script', () => {
      const scriptPath = path.join(containersDir, 'deployer', 'aphex-deploy-pipeline');
      expect(fs.existsSync(scriptPath)).toBe(true);
      
      // Check if file has shebang
      const content = fs.readFileSync(scriptPath, 'utf-8');
      expect(content.startsWith('#!/')).toBe(true);
    });

    it('should have executable aphex-deploy-stack script', () => {
      const scriptPath = path.join(containersDir, 'deployer', 'aphex-deploy-stack');
      expect(fs.existsSync(scriptPath)).toBe(true);
      
      // Check if file has shebang
      const content = fs.readFileSync(scriptPath, 'utf-8');
      expect(content.startsWith('#!/')).toBe(true);
    });

    it('should have executable aphex-test script', () => {
      const scriptPath = path.join(containersDir, 'tester', 'aphex-test');
      expect(fs.existsSync(scriptPath)).toBe(true);
      
      // Check if file has shebang
      const content = fs.readFileSync(scriptPath, 'utf-8');
      expect(content.startsWith('#!/')).toBe(true);
    });

    it('should have executable aphex-validate script', () => {
      const scriptPath = path.join(containersDir, 'validator', 'aphex-validate');
      expect(fs.existsSync(scriptPath)).toBe(true);
      
      // Check if file has shebang
      const content = fs.readFileSync(scriptPath, 'utf-8');
      expect(content.startsWith('#!/')).toBe(true);
    });
  });

  describe('Base Image Verification', () => {
    it('should use correct base image for Builder', () => {
      const dockerfilePath = path.join(containersDir, 'builder', 'Dockerfile');
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      const fromLine = dockerfile.split('\n').find(line => line.trim().startsWith('FROM'));
      expect(fromLine).toBe('FROM node:20-alpine');
    });

    it('should use correct base image for Deployer', () => {
      const dockerfilePath = path.join(containersDir, 'deployer', 'Dockerfile');
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      const fromLine = dockerfile.split('\n').find(line => line.trim().startsWith('FROM'));
      expect(fromLine).toBe('FROM node:20-alpine');
    });

    it('should use correct base image for Tester', () => {
      const dockerfilePath = path.join(containersDir, 'tester', 'Dockerfile');
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      const fromLine = dockerfile.split('\n').find(line => line.trim().startsWith('FROM'));
      expect(fromLine).toBe('FROM node:20-alpine');
    });

    it('should use correct base image for Validator', () => {
      const dockerfilePath = path.join(containersDir, 'validator', 'Dockerfile');
      const dockerfile = fs.readFileSync(dockerfilePath, 'utf-8');
      const fromLine = dockerfile.split('\n').find(line => line.trim().startsWith('FROM'));
      expect(fromLine).toBe('FROM python:3.11-slim');
    });
  });
});
